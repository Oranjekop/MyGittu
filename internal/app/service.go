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
	return s.writeLocked(ctx, id, func() (git.Result, error) { return s.Read(ctx, id, args...) })
}
func (s *Service) writeLocked(ctx context.Context, id string, work func() (git.Result, error)) (git.Result, error) {
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
	return work()
}
func (s *Service) Status(ctx context.Context, id string) ([]git.FileStatus, error) {
	files, _, e := s.WorkspaceStatus(ctx, id)
	return files, e
}

type BranchStatus struct {
	Name     string
	Upstream string
	Ahead    int
	Behind   int
	Compared bool
}

func (s *Service) WorkspaceStatus(ctx context.Context, id string) ([]git.FileStatus, BranchStatus, error) {
	var branch BranchStatus
	r, e := s.Read(ctx, id, "status", "--porcelain=v2", "--branch", "-z", "--untracked-files=all", "--ignored=matching")
	if e != nil {
		return nil, branch, e
	}
	for headers := r.Stdout; bytes.HasPrefix(headers, []byte("# ")); {
		line, rest, ok := bytes.Cut(headers, []byte{0})
		if !ok {
			break
		}
		headers = rest
		key, value, _ := strings.Cut(string(line[2:]), " ")
		switch key {
		case "branch.head":
			branch.Name = value
		case "branch.upstream":
			branch.Upstream = value
		case "branch.ab":
			ahead, behind, _ := strings.Cut(value, " ")
			var aheadErr, behindErr error
			branch.Ahead, aheadErr = strconv.Atoi(strings.TrimPrefix(ahead, "+"))
			branch.Behind, behindErr = strconv.Atoi(strings.TrimPrefix(behind, "-"))
			branch.Compared = aheadErr == nil && behindErr == nil
		}
	}
	if branch.Name == "(detached)" {
		branch.Name = "detached HEAD"
	}
	files, e := git.ParseStatus(r.Stdout)
	return files, branch, e
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
	Hash    string   `json:"hash"`
	Author  string   `json:"author"`
	Date    string   `json:"date"`
	Subject string   `json:"subject"`
	Parents []string `json:"parents,omitempty"`
	Refs    string   `json:"refs,omitempty"`
}

func (s *Service) Log(ctx context.Context, id string, limit, offset int) ([]Commit, error) {
	return s.LogSearch(ctx, id, limit, offset, "")
}
func (s *Service) LogSearch(ctx context.Context, id string, limit, offset int, query string) ([]Commit, error) {
	return s.History(ctx, id, limit, offset, query, false)
}

func (s *Service) History(ctx context.Context, id string, limit, offset int, query string, allBranches bool) ([]Commit, error) {
	return s.log(ctx, id, limit, offset, query, "", allBranches)
}
func (s *Service) FileLog(ctx context.Context, id, path string, limit, offset int) ([]Commit, error) {
	repo, err := s.Registry.Get(id)
	if err != nil {
		return nil, err
	}
	path, err = repository.Relative(repo.Root, path)
	if err != nil {
		return nil, err
	}
	return s.log(ctx, id, limit, offset, "", path, false)
}
func (s *Service) log(ctx context.Context, id string, limit, offset int, query, path string, allBranches bool) ([]Commit, error) {
	if limit < 1 || limit > 500 || offset < 0 {
		return nil, fmt.Errorf("invalid log page")
	}
	if len(query) > 1024 {
		return nil, fmt.Errorf("search text too long")
	}
	args := []string{"log", "-z", "--format=%H%x00%an%x00%aI%x00%s%x00%P%x00%D", "--decorate=short", "--max-count=" + strconv.Itoa(limit), "--skip=" + strconv.Itoa(offset)}
	if path == "" {
		args = append(args, "--topo-order")
	}
	if allBranches {
		args = append(args, "--branches", "--ignore-missing", "HEAD")
	}
	if query != "" {
		args = append(args, "--fixed-strings", "--regexp-ignore-case", "--grep="+query)
	}
	if path != "" {
		args = append(args, "--follow", "--", path)
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
	for i := 0; i+5 < len(fields); i += 6 {
		out = append(out, Commit{Hash: string(fields[i]), Author: string(fields[i+1]), Date: string(fields[i+2]),
			Subject: string(fields[i+3]), Parents: strings.Fields(string(fields[i+4])), Refs: string(fields[i+5])})
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

type CommitDetails struct {
	Summary string `json:"summary"`
	Patch   string `json:"patch"`
}

func (s *Service) CommitDetails(ctx context.Context, id, hash string) (CommitDetails, error) {
	if !commitHash.MatchString(hash) {
		return CommitDetails{}, fmt.Errorf("invalid commit hash")
	}
	summary, err := s.Read(ctx, id, "show", "--no-patch", "--format=%B", hash, "--")
	if err != nil {
		return CommitDetails{}, err
	}
	patch, err := s.Read(ctx, id, "show", "--format=", "--patch", "--diff-merges=first-parent",
		"--no-ext-diff", "--no-textconv", "--no-color", "--src-prefix=a/", "--dst-prefix=b/", hash, "--")
	return CommitDetails{Summary: strings.TrimSpace(string(summary.Stdout)), Patch: string(patch.Stdout)}, err
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
	_, err := s.RemoteWithResult(ctx, id, action)
	return err
}

type RemoteResult struct {
	NothingToPush bool `json:"nothing_to_push"`
	UpToDate      bool `json:"up_to_date,omitempty"`
}

func (s *Service) RemoteWithResult(ctx context.Context, id, action string) (RemoteResult, error) {
	var args []string
	switch action {
	case "fetch":
		args = []string{"fetch", "--all", "--prune"}
	case "pull":
		args = []string{"pull", "--ff-only"}
	case "push":
		args = []string{"push", "--porcelain"}
	default:
		return RemoteResult{}, fmt.Errorf("unsupported remote operation")
	}
	if action != "push" {
		probe := []string{"rev-parse", "--verify", "HEAD"}
		if action == "fetch" {
			probe = []string{"for-each-ref", "--sort=refname", "--format=%(refname) %(objectname)"}
		}
		unchanged := false
		_, err := s.writeLocked(ctx, id, func() (git.Result, error) {
			before, beforeErr := s.Read(ctx, id, probe...)
			result, err := s.Read(ctx, id, args...)
			if err == nil && beforeErr == nil {
				after, afterErr := s.Read(ctx, id, probe...)
				unchanged = afterErr == nil && bytes.Equal(before.Stdout, after.Stdout)
			}
			return result, err
		})
		return RemoteResult{UpToDate: unchanged}, err
	}
	result, err := s.Write(ctx, id, args...)
	if err != nil {
		return RemoteResult{}, err
	}
	// Git porcelain reports one tab-delimited row per ref; '=' is up to date.
	seen := false
	for line := range strings.SplitSeq(string(result.Stdout), "\n") {
		flag, rest, ok := strings.Cut(line, "\t")
		if !ok || len(flag) != 1 || !strings.Contains(rest, "\t") {
			continue
		}
		if flag != "=" {
			return RemoteResult{}, nil
		}
		seen = true
	}
	return RemoteResult{NothingToPush: seen}, nil
}
func (s *Service) Clone(ctx context.Context, url, destination string, credentials ...git.CloneAuth) (repository.Info, error) {
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
	var auth git.CloneAuth
	if len(credentials) > 0 {
		auth = credentials[0]
	}
	if e := s.Runner.Clone(ctx, url, destination, auth); e != nil {
		return repository.Info{}, e
	}
	return s.Registry.Register(ctx, destination)
}
