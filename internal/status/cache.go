package status

import (
	"context"
	"errors"
	"maps"
	"mygittu/internal/app"
	git "mygittu/internal/git"
	"mygittu/internal/platform"
	"mygittu/internal/repository"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Overlay struct {
	State      git.State `json:"state"`
	Generation uint64    `json:"generation"`
	Cached     bool      `json:"cached"`
}

var ErrStale = errors.New("repository changed during refresh; retry")

type Snapshot struct {
	Files         []git.FileStatus `json:"files"`
	Generation    uint64           `json:"generation"`
	Updated       time.Time        `json:"updated"`
	Branch        string           `json:"branch"`
	CherryPicking bool             `json:"cherry_picking"`
	Merging       bool             `json:"merging"`
	Tracking      app.BranchStatus `json:"tracking"`
	states        map[string]git.State
}
type Cache struct {
	mu         sync.RWMutex
	snapshots  map[string]Snapshot
	refresh    sync.Map
	Service    *app.Service
	generation uint64
	epochs     map[string]uint64
	queued     map[string]struct{}
}

func New(s *app.Service) *Cache {
	return &Cache{snapshots: map[string]Snapshot{}, epochs: map[string]uint64{}, queued: map[string]struct{}{}, Service: s}
}
func (c *Cache) Refresh(ctx context.Context, id string) error {
	if _, err := c.Service.Registry.Get(id); err != nil {
		return err
	}
	v, _ := c.refresh.LoadOrStore(id, make(chan struct{}, 1))
	m := v.(chan struct{})
	select {
	case m <- struct{}{}:
		defer func() { <-m }()
	case <-ctx.Done():
		return ctx.Err()
	}
	c.mu.RLock()
	epoch := c.epochs[id]
	c.mu.RUnlock()
	repo, e := c.Service.Registry.Get(id)
	if e != nil {
		return e
	}
	files, branch, e := c.Service.WorkspaceStatus(ctx, id)
	if e != nil {
		return e
	}
	states := map[string]git.State{".": git.Clean}
	for _, f := range files {
		p := repository.Key(filepath.FromSlash(f.Path))
		states[p] = git.Aggregate(states[p], f.State)
		if f.State == git.Ignored {
			continue
		}
		for p = filepath.Dir(p); ; p = filepath.Dir(p) {
			states[p] = git.Aggregate(states[p], f.State)
			if p == "." {
				break
			}
		}
	}
	c.mu.Lock()
	if c.epochs[id] != epoch {
		c.mu.Unlock()
		return ErrStale
	}
	changed := !maps.Equal(c.snapshots[id].states, states)
	c.generation++
	_, mergeErr := os.Stat(filepath.Join(repo.GitDir, "MERGE_HEAD"))
	_, cherryErr := os.Stat(filepath.Join(repo.GitDir, "CHERRY_PICK_HEAD"))
	cherryPicking := cherryErr == nil
	c.snapshots[id] = Snapshot{Files: files, Generation: c.generation, Updated: time.Now(), Branch: branch.Name, Merging: mergeErr == nil, CherryPicking: cherryPicking, Tracking: branch, states: states}
	c.mu.Unlock()
	if changed {
		// Let Explorer's 500 ms native lookup cache expire before repainting.
		time.AfterFunc(550*time.Millisecond, func() {
			platform.NotifyRepositoryOverlay(repo.Root)
		})
	}
	return nil
}
func (c *Cache) Invalidate(id string) {
	c.mu.Lock()
	c.epochs[id]++
	delete(c.snapshots, id)
	delete(c.queued, id)
	c.mu.Unlock()
}

// ScheduleRefresh invalidates in-flight scans and queues one replacement per repository.
func (c *Cache) ScheduleRefresh(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := c.Service.Registry.Get(id); err != nil {
		return
	}
	c.epochs[id]++
	c.queued[id] = struct{}{}
}

func (c *Cache) takeScheduled() map[string]struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.queued) == 0 {
		return nil
	}
	queued := c.queued
	c.queued = make(map[string]struct{})
	return queued
}

// A filesystem event invalidates in-flight scans. Keep the last successful
// snapshot visible until a replacement is ready.
func (c *Cache) markDirty(id string) {
	c.mu.Lock()
	c.epochs[id]++
	c.mu.Unlock()
}
func (c *Cache) Get(id string) (Snapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.snapshots[id]
	v.Files = append([]git.FileStatus{}, v.Files...)
	return v, ok
}

func (c *Cache) GetSince(id string, previous time.Time) Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	snapshot := c.snapshots[id]
	if !snapshot.Updated.After(previous) {
		return Snapshot{}
	}
	snapshot.Files = append([]git.FileStatus(nil), snapshot.Files...)
	return snapshot
}
func (c *Cache) Lookup(path string) Overlay {
	if !filepath.IsAbs(path) {
		return Overlay{State: git.Unknown}
	}
	repos := c.Service.Registry.List()
	best := ""
	id := ""
	for _, r := range repos {
		for _, root := range append([]string{r.Root}, r.Aliases...) {
			rel, e := filepath.Rel(root, path)
			if e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && len(root) > len(best) {
				best = root
				id = r.ID
			}
		}
	}
	if id == "" {
		return Overlay{State: git.Unknown}
	}
	rel, _ := filepath.Rel(best, path)
	rel = repository.Key(rel)
	if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
		return Overlay{State: git.Unknown}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.snapshots[id]
	if !ok {
		return Overlay{State: git.Unknown}
	}
	state, ok := s.states[rel]
	if !ok {
		state = git.Clean
		for p := filepath.Dir(rel); p != "."; p = filepath.Dir(p) {
			if s.states[p] == git.Ignored {
				state = git.Ignored
				break
			}
		}
	}
	return Overlay{state, s.Generation, true}
}
