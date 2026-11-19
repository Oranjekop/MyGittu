package status

import (
	"context"
	"errors"
	"io/fs"
	"mygittu/internal/repository"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

// One scan worker bounds concurrent Git processes. Events coalesce by repository;
// events during a scan queue a follow-up without discarding that snapshot.
// Watch count is capped; other repositories still reconcile every 30 seconds.
func (c *Cache) Run(ctx context.Context) {
	c.watch(ctx)
	if ctx.Err() == nil {
		c.poll(ctx)
	}
}

func (c *Cache) watch(ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	defer watcher.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	budget := newWatchBudget()
	work := make(chan string)
	completed := make(chan string, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case id := <-work:
				_ = c.Refresh(ctx, id)
				select {
				case completed <- id:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	pending := map[string]time.Time{}
	known := map[string]repository.Info{}
	watched := map[string]string{}
	busy := false
	addTree := func(id, root string) error {
		return filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
			if ctx.Err() != nil {
				return fs.SkipAll
			}
			if len(watched) >= 2048 {
				return fs.SkipAll
			}
			if e != nil {
				return nil
			}
			if !d.IsDir() {
				return nil
			}
			if d.Name() == ".git" && p != root {
				return fs.SkipDir
			}
			if _, ok := watched[p]; !ok {
				if !budget.takeDirectory(p) {
					return errWatchBudget
				}
				if err := watcher.Add(p); err == nil {
					watched[p] = id
				} else if descriptorLimit(err) {
					return err
				}
			}
			return nil
		})
	}
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	reconcile := time.NewTicker(30 * time.Second)
	defer reconcile.Stop()
	registryTick := time.NewTicker(time.Second)
	defer registryTick.Stop()
	syncRepos := func() error {
		repos := c.Service.Registry.List()
		active := map[string]bool{}
		for _, r := range repos {
			active[r.ID] = true
			if _, ok := known[r.ID]; !ok {
				known[r.ID] = r
				if err := addTree(r.ID, r.Root); err != nil {
					return err
				}
				if err := addTree(r.ID, r.GitDir); err != nil {
					return err
				}
				pending[r.ID] = time.Now()
			}
		}
		for id := range known {
			if !active[id] {
				delete(known, id)
				delete(pending, id)
				c.Invalidate(id)
				for p, owner := range watched {
					if owner == id {
						_ = watcher.Remove(p)
						delete(watched, p)
					}
				}
			}
		}
		return nil
	}
	if err := syncRepos(); err != nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-registryTick.C:
			if err := syncRepos(); err != nil {
				return
			}
		case <-reconcile.C:
			for id := range known {
				pending[id] = time.Now()
			}
		case err, ok := <-watcher.Errors:
			if !ok || descriptorLimit(err) {
				return
			}
			for id := range known {
				pending[id] = time.Now()
			}
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			id := watched[filepath.Dir(event.Name)]
			if id == "" {
				id = watched[event.Name]
			}
			if id != "" {
				if _, scheduled := pending[id]; !scheduled {
					pending[id] = time.Now().Add(250 * time.Millisecond)
				}
				if event.Op&fsnotify.Create != 0 {
					if !budget.takeEntry() {
						return
					}
					if st, e := os.Stat(event.Name); e == nil && st.IsDir() {
						if err := addTree(id, event.Name); err != nil {
							return
						}
					}
				}
				if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
					delete(watched, event.Name)
				}
			}
		case <-completed:
			busy = false
		case now := <-tick.C:
			for id := range c.takeScheduled() {
				if _, exists := pending[id]; !exists {
					pending[id] = now
				}
			}
			if !busy {
				for id, when := range pending {
					if time.Now().After(when) {
						select {
						case work <- id:
							busy = true
							delete(pending, id)
						case <-ctx.Done():
							return
						}
						break
					}
				}
			}
		}
	}
}

var errWatchBudget = errors.New("filesystem watch budget exhausted")

func descriptorLimit(err error) bool {
	return errors.Is(err, syscall.EMFILE) || errors.Is(err, syscall.ENFILE)
}

func (c *Cache) poll(ctx context.Context) {
	const reconcileInterval = 5 * time.Second
	tick := time.NewTimer(0)
	defer tick.Stop()
	scheduled := time.NewTicker(200 * time.Millisecond)
	defer scheduled.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			for _, r := range c.Service.Registry.List() {
				if ctx.Err() != nil {
					return
				}
				_ = c.Refresh(ctx, r.ID)
			}
			tick.Reset(reconcileInterval)
		case <-scheduled.C:
			for id := range c.takeScheduled() {
				_ = c.Refresh(ctx, id)
			}
		}
	}
}
