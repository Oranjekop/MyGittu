package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	git "mygittu/internal/git"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

type Info struct {
	ID         string   `json:"id"`
	Root       string   `json:"root"`
	GitDir     string   `json:"git_dir"`
	Aliases    []string `json:"aliases,omitempty"`
	LastOpened int64    `json:"last_opened,omitempty"`
}
type Registry struct {
	mu     sync.RWMutex
	repos  map[string]Info
	file   string
	runner *git.Runner
}

func New(r *git.Runner, file string) (*Registry, error) {
	x := &Registry{repos: map[string]Info{}, file: file, runner: r}
	if file != "" {
		b, e := os.ReadFile(file)
		if e == nil {
			var list []Info
			if e = json.Unmarshal(b, &list); e != nil {
				return nil, e
			}
			for _, v := range list {
				info, e := x.Discover(context.Background(), v.Root)
				if e == nil {
					info.LastOpened = v.LastOpened
					info.Aliases = mergeAliases(info.Aliases, v.Aliases)
					x.repos[info.ID] = info
				}
			}
		} else if !os.IsNotExist(e) {
			return nil, e
		}
	}
	return x, nil
}
func (r *Registry) Discover(ctx context.Context, path string) (Info, error) {
	p, e := filepath.Abs(path)
	if e != nil {
		return Info{}, e
	}
	s, e := os.Stat(p)
	if e != nil {
		return Info{}, e
	}
	if !s.IsDir() {
		p = filepath.Dir(p)
	}
	v, e := r.runner.Run(ctx, p, "rev-parse", "--show-toplevel", "--absolute-git-dir", "--show-prefix")
	if e != nil {
		return Info{}, e
	}
	lines := strings.Split(strings.TrimSuffix(string(v.Stdout), "\n"), "\n")
	if len(lines) != 3 {
		return Info{}, fmt.Errorf("unsupported repository root path")
	}
	root, e := filepath.EvalSymlinks(filepath.FromSlash(strings.TrimSuffix(lines[0], "\r")))
	if e != nil {
		return Info{}, e
	}
	alias := p
	for _, part := range strings.Split(strings.Trim(lines[2], "/\r"), "/") {
		if part != "" {
			alias = filepath.Dir(alias)
		}
	}
	hash := sha256.Sum256([]byte(Key(root)))
	return Info{ID: hex.EncodeToString(hash[:12]), Root: root, GitDir: filepath.FromSlash(strings.TrimSuffix(lines[1], "\r")), Aliases: []string{alias}}, nil
}

// Key performs lexical normalization only, so Explorer lookups never touch disk.
func Key(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	return path
}
func (r *Registry) Register(ctx context.Context, path string) (Info, error) {
	return r.register(ctx, path, true)
}
func (r *Registry) RegisterDiscovered(ctx context.Context, path string) (Info, error) {
	return r.register(ctx, path, false)
}
func (r *Registry) register(ctx context.Context, path string, opened bool) (Info, error) {
	info, e := r.Discover(ctx, path)
	if e != nil {
		return info, e
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	old, exists := r.repos[info.ID]
	if exists {
		info.Aliases = mergeAliases(info.Aliases, old.Aliases)
		info.LastOpened = old.LastOpened
	}
	if opened {
		info.LastOpened = time.Now().UnixNano()
	}
	if !exists && len(r.repos) >= 256 {
		return info, fmt.Errorf("repository registration limit reached (256)")
	}
	r.repos[info.ID] = info
	if e = r.save(); e != nil {
		if exists {
			r.repos[info.ID] = old
		} else {
			delete(r.repos, info.ID)
		}
	}
	return info, e
}
func mergeAliases(a, b []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, p := range append(append([]string{}, a...), b...) {
		key := Key(p)
		if !seen[key] {
			seen[key] = true
			out = append(out, p)
		}
	}
	return out
}
func (r *Registry) Unregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	old, ok := r.repos[id]
	delete(r.repos, id)
	if e := r.save(); e != nil {
		if ok {
			r.repos[id] = old
		}
		return e
	}
	return nil
}
func (r *Registry) Get(id string) (Info, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.repos[id]
	if !ok {
		return v, fmt.Errorf("repository not registered")
	}
	return v, nil
}
func (r *Registry) FindPath(path string) (Info, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	best := Info{}
	found := false
	for _, v := range r.repos {
		for _, root := range append([]string{v.Root}, v.Aliases...) {
			rel, e := filepath.Rel(root, path)
			if e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && (!found || len(root) > len(best.Root)) {
				best = v
				found = true
			}
		}
	}
	return best, found
}

// FindGitRoot checks the lightweight repository marker without invoking Git.
// Explorer can use this path for folders that the user has never opened.
func FindGitRoot(path string) (string, bool) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(absolute)
	if err == nil && !info.IsDir() {
		absolute = filepath.Dir(absolute)
	}
	for depth := 0; depth < 64; depth++ {
		marker := filepath.Join(absolute, ".git")
		if _, err := os.Stat(marker); err == nil {
			return absolute, true
		}
		parent := filepath.Dir(absolute)
		if parent == absolute {
			break
		}
		absolute = parent
	}
	return "", false
}
func (r *Registry) List() []Info { r.mu.RLock(); defer r.mu.RUnlock(); return r.list() }
func (r *Registry) list() []Info {
	a := make([]Info, 0, len(r.repos))
	for _, v := range r.repos {
		a = append(a, v)
	}
	sort.Slice(a, func(i, j int) bool {
		if a[i].LastOpened != a[j].LastOpened {
			return a[i].LastOpened > a[j].LastOpened
		}
		return a[i].Root < a[j].Root
	})
	return a
}
func (r *Registry) save() error {
	if r.file == "" {
		return nil
	}
	b, e := json.Marshal(r.list())
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(r.file), 0700); e != nil {
		return e
	}
	tmp := r.file + ".tmp"
	if e = os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, r.file)
}
func Relative(root, path string) (string, error) {
	if filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return "", fmt.Errorf("relative path required")
	}
	p := filepath.Clean(path)
	if p == "." || p == ".." || strings.HasPrefix(p, ".."+string(filepath.Separator)) || strings.ContainsRune(p, 0) {
		return "", fmt.Errorf("invalid repository path")
	}
	return filepath.ToSlash(p), nil
}
