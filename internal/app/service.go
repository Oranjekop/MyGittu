package app

import (
	"bytes"
	"context"
	"fmt"
	git "mygittu/internal/git"
	"mygittu/internal/repository"
	"mygittu/internal/settings"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type Service struct {
	Runner   *git.Runner
	Registry *repository.Registry
	Settings *settings.Store
	locks    sync.Map
}

func (s *Service) Read(ctx context.Context, id string, args ...string) (git.Result, error) {
	r, e := s.Registry.Get(id)
	if e != nil {
		return git.Result{}, e
	}
	return s.Runner.Run(ctx, r.Root, args...)
}
func (s *Service) Write(ctx context.Context, id string, args ...string) (git.Result, error) {
	if _, e := s.Registry.Get(id); e != nil {
		return git.Result{}, e
	}
	v, _ := s.locks.LoadOrStore(id, make(chan struct{}, 1))
	lock := v.(chan struct{})
	select {
	case lock <- struct{}{}:
		defer func() { <-lock }()
	case <-ctx.Done():
		return git.Result{}, ctx.Err()
	}
	return s.Read(ctx, id, args...)
}
func (s *Service) Status(ctx context.Context, id string) ([]git.FileStatus, error) {
	r, e := s.Read(ctx, id, "status", "--porcelain=v2", "-z", "--untracked-files=all", "--ignored=matching")
	if e != nil {
		return nil, e
	}
	return git.ParseStatus(r.Stdout)
}
func (s *Service) Paths(ctx context.Context, id string, paths []string, stage bool) error {
	if len(paths) == 0 {
		return fmt.Errorf("select at least one file")
	}
	repo, e := s.Registry.Get(id)
	if e != nil {
		return e
	}
	clean := make([]string, len(paths))
	for i, p := range paths {
		clean[i], e = repository.Relative(repo.Root, p)
		if e != nil {
			return e
		}
	}
	args := []string{"add", "--"}
	if !stage {
		_, e := s.Read(ctx, id, "rev-parse", "--verify", "HEAD")
		if e != nil {
			args = []string{"rm", "--cached", "--ignore-unmatch", "--"}
		} else {
			args = []string{"restore", "--staged", "--"}
		}
	}
	_, e = s.Write(ctx, id, append(args, clean...)...)
	return e
}
func (s *Service) Commit(ctx context.Context, id, message string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", fmt.Errorf("commit message required")
	}
	if len(message) > 1<<20 {
		return "", fmt.Errorf("commit message too large")
	}
	_, e := s.Write(ctx, id, "commit", "-m", message)
	if e != nil {
		return "", e
	}
	r, e := s.Read(ctx, id, "rev-parse", "HEAD")
	return strings.TrimSpace(string(r.Stdout)), e
}

type Commit struct {
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Subject string `json:"subject"`
}

func (s *Service) Log(ctx context.Context, id string, limit, offset int) ([]Commit, error) {
	return s.LogSearch(ctx, id, limit, offset, "")
}
func (s *Service) LogSearch(ctx context.Context, id string, limit, offset int, query string) ([]Commit, error) {
	if limit < 1 || limit > 500 || offset < 0 {
		return nil, fmt.Errorf("invalid log page")
	}
	if len(query) > 1024 {
		return nil, fmt.Errorf("search text too long")
	}
	args := []string{"log", "-z", "--format=%H%x00%an%x00%aI%x00%s", "--max-count=" + strconv.Itoa(limit), "--skip=" + strconv.Itoa(offset)}
	if query != "" {
		args = append(args, "--fixed-strings", "--regexp-ignore-case", "--grep="+query)
	}
	r, e := s.Read(ctx, id, args...)
	if e != nil {
		if _, err := s.Read(ctx, id, "rev-parse", "--verify", "HEAD"); err != nil {
			return []Commit{}, nil
		}
		return nil, e
	}
	fields := bytes.Split(bytes.TrimSuffix(r.Stdout, []byte{0}), []byte{0})
	out := []Commit{}
	for i := 0; i+3 < len(fields); i += 4 {
		out = append(out, Commit{string(fields[i]), string(fields[i+1]), string(fields[i+2]), string(fields[i+3])})
	}
	return out, nil
}

var commitHash = regexp.MustCompile(`^[a-fA-F0-9]{40}([a-fA-F0-9]{24})?$`)

func (s *Service) Show(ctx context.Context, id, hash string) (string, error) {
	if !commitHash.MatchString(hash) {
		return "", fmt.Errorf("invalid commit hash")
	}
	r, e := s.Read(ctx, id, "show", "--no-ext-diff", "--no-textconv", "--no-color", "--format=fuller", "--stat", hash, "--")
	return string(r.Stdout), e
}
func (s *Service) Diff(ctx context.Context, id, path string, staged bool) (string, error) {
	repo, e := s.Registry.Get(id)
	if e != nil {
		return "", e
	}
	args := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-color"}
	if staged {
		args = append(args, "--cached")
	}
	if path != "" {
		path, e = repository.Relative(repo.Root, path)
		if e != nil {
			return "", e
		}
		args = append(args, "--", path)
	}
	r, e := s.Read(ctx, id, args...)
	return string(r.Stdout), e
}
func (s *Service) Branches(ctx context.Context, id string) (string, error) {
	r, e := s.Read(ctx, id, "branch", "--no-color", "-v")
	return string(r.Stdout), e
}
func (s *Service) Branch(ctx context.Context, id, action, name string) error {
	if name == "" || strings.HasPrefix(name, "-") {
		return fmt.Errorf("branch name required")
	}
	if _, e := s.Read(ctx, id, "check-ref-format", "--branch", name); e != nil {
		return fmt.Errorf("invalid branch name")
	}
	var args []string
	switch action {
	case "create":
		args = []string{"branch", name}
	case "switch":
		args = []string{"switch", name}
	case "delete":
		args = []string{"branch", "-d", name}
	case "merge":
		args = []string{"merge", "--no-edit", name}
	default:
		return fmt.Errorf("unsupported branch operation")
	}
	_, e := s.Write(ctx, id, args...)
	return e
}
func (s *Service) Remote(ctx context.Context, id, action string) error {
	var args []string
	switch action {
	case "fetch":
		args = []string{"fetch", "--all", "--prune"}
	case "pull":
		args = []string{"pull", "--ff-only"}
	case "push":
		args = []string{"push"}
	default:
		return fmt.Errorf("unsupported remote operation")
	}
	_, e := s.Write(ctx, id, args...)
	return e
}
func (s *Service) Clone(ctx context.Context, url, destination string) (repository.Info, error) {
	if strings.TrimSpace(url) == "" || strings.HasPrefix(url, "-") || strings.Contains(url, "::") {
		return repository.Info{}, fmt.Errorf("clone URL required")
	}
	if filepath.IsAbs(destination) == false {
		return repository.Info{}, fmt.Errorf("absolute destination required")
	}
	if _, e := os.Stat(destination); e == nil {
		entries, err := os.ReadDir(destination)
		if err != nil || len(entries) > 0 {
			return repository.Info{}, fmt.Errorf("destination must be an empty folder")
		}
	} else if !os.IsNotExist(e) {
		return repository.Info{}, e
	}
	if e := os.MkdirAll(filepath.Dir(destination), 0700); e != nil {
		return repository.Info{}, e
	}
	if _, e := s.Runner.Run(ctx, "", "clone", "--", url, destination); e != nil {
		return repository.Info{}, e
	}
	return s.Registry.Register(ctx, destination)
}
