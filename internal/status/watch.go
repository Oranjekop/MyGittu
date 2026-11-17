package status

import (
	"context"
	"github.com/fsnotify/fsnotify"
	"io/fs"
	"mygittu/internal/repository"
	"os"
	"path/filepath"
	"time"
)

// One scan worker bounds concurrent Git processes. Events debounce by repository;
// overflow/errors fall back to reconciliation. Watch count is capped to bound OS
// resources; repositories beyond that cap still reconcile every 30 seconds.
func (c *Cache) Run(ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		c.poll(ctx)
		return
	}
	defer watcher.Close()
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
	addTree := func(id, root string) {
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
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
				if watcher.Add(p) == nil {
					watched[p] = id
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
	syncRepos := func() {
		repos := c.Service.Registry.List()
		active := map[string]bool{}
		for _, r := range repos {
			active[r.ID] = true
			if _, ok := known[r.ID]; !ok {
				known[r.ID] = r
				addTree(r.ID, r.Root)
				addTree(r.ID, r.GitDir)
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
	}
	syncRepos()
	for {
		select {
		case <-ctx.Done():
			return
		case <-registryTick.C:
			syncRepos()
		case <-reconcile.C:
			for id := range known {
				pending[id] = time.Now()
			}
		case <-watcher.Errors:
			for id := range known {
				c.Invalidate(id)
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
				c.Invalidate(id)
				pending[id] = time.Now().Add(250 * time.Millisecond)
				if event.Op&fsnotify.Create != 0 {
					if st, e := os.Stat(event.Name); e == nil && st.IsDir() {
						addTree(id, event.Name)
					}
				}
				if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
					delete(watched, event.Name)
				}
			}
		case <-completed:
			busy = false
		case <-tick.C:
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
func (c *Cache) poll(ctx context.Context) {
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		for _, r := range c.Service.Registry.List() {
			if ctx.Err() != nil {
				return
			}
			_ = c.Refresh(ctx, r.ID)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
