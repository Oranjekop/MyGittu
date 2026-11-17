package status

import (
	"context"
	"errors"
	"mygittu/internal/app"
	git "mygittu/internal/git"
	"mygittu/internal/repository"
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
	Files      []git.FileStatus `json:"files"`
	Generation uint64           `json:"generation"`
	Updated    time.Time        `json:"updated"`
	Branch     string           `json:"branch"`
	states     map[string]git.State
}
type Cache struct {
	mu         sync.RWMutex
	snapshots  map[string]Snapshot
	refresh    sync.Map
	Service    *app.Service
	generation uint64
	epochs     map[string]uint64
}

func New(s *app.Service) *Cache {
	return &Cache{snapshots: map[string]Snapshot{}, epochs: map[string]uint64{}, Service: s}
}
func (c *Cache) Refresh(ctx context.Context, id string) error {
	if _, e := c.Service.Registry.Get(id); e != nil {
		return e
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
	files, e := c.Service.Status(ctx, id)
	if e != nil {
		c.Invalidate(id)
		return e
	}
	branchResult, _ := c.Service.Read(ctx, id, "symbolic-ref", "--quiet", "--short", "HEAD")
	branch := strings.TrimSpace(string(branchResult.Stdout))
	if branch == "" {
		branch = "detached HEAD"
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
	c.generation++
	c.snapshots[id] = Snapshot{Files: files, Generation: c.generation, Updated: time.Now(), Branch: branch, states: states}
	c.mu.Unlock()
	return nil
}
func (c *Cache) Invalidate(id string) {
	c.mu.Lock()
	c.epochs[id]++
	delete(c.snapshots, id)
	c.mu.Unlock()
}
func (c *Cache) Get(id string) (Snapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.snapshots[id]
	v.Files = append([]git.FileStatus{}, v.Files...)
	return v, ok
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
