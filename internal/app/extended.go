package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	git "mygittu/internal/git"
	"mygittu/internal/repository"
)

type RemoteEntry struct{ Name, URL string }

func (s *Service) Remotes(ctx context.Context, id string) ([]RemoteEntry, error) {
	result, err := s.Read(ctx, id, "remote", "-v")
	if err != nil {
		return nil, err
	}
	var remotes []RemoteEntry
	for line := range strings.SplitSeq(string(result.Stdout), "\n") {
		name, value, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if ok && strings.HasSuffix(value, " (fetch)") {
			remotes = append(remotes, RemoteEntry{name, git.Redact(strings.TrimSuffix(value, " (fetch)"))})
		}
	}
	return remotes, nil
}

func (s *Service) ConfigureRemote(ctx context.Context, id, action, name, value string) error {
	if name == "" || strings.HasPrefix(name, "-") || strings.ContainsAny(name, " \t\r\n") {
		return fmt.Errorf("invalid remote name")
	}
	if _, err := s.Read(ctx, id, "check-ref-format", "refs/remotes/"+name+"/HEAD"); err != nil {
		return err
	}
	var args []string
	switch action {
	case "add", "set-url":
		if value == "" || strings.HasPrefix(value, "-") || strings.Contains(value, "::") || strings.ContainsAny(value, "\r\n") || git.Redact(value) != value {
			return fmt.Errorf("invalid remote URL or embedded credentials")
		}
		if parsed, err := url.Parse(value); err == nil && parsed.User != nil {
			if _, hasPassword := parsed.User.Password(); hasPassword {
				return fmt.Errorf("enter credentials separately from the remote address")
			}
		}
		args = []string{"remote", action, "--", name, value}
	case "remove":
		args = []string{"remote", "remove", "--", name}
	case "fetch":
		args = []string{"fetch", "--prune", "--", name}
	case "pull", "track", "push":
		if value == "" || strings.HasPrefix(value, "-") {
			return fmt.Errorf("enter a branch name")
		}
		if _, err := s.Read(ctx, id, "check-ref-format", "refs/heads/"+value); err != nil {
			return err
		}
		switch action {
		case "pull":
			args = []string{"pull", "--ff-only", "--", name, "refs/heads/" + value}
		case "track":
			args = []string{"branch", "--set-upstream-to=refs/remotes/" + name + "/" + value}
		case "push":
			_, err := s.writeLocked(ctx, id, func() (git.Result, error) {
				head, err := s.Read(ctx, id, "symbolic-ref", "--quiet", "HEAD")
				if err != nil {
					return git.Result{}, fmt.Errorf("check out a local branch before setting upstream")
				}
				return s.Read(ctx, id, "push", "--set-upstream", "--", name, strings.TrimSpace(string(head.Stdout))+":refs/heads/"+value)
			})
			return err
		}
	default:
		return fmt.Errorf("unknown remote action")
	}
	_, err := s.Write(ctx, id, args...)
	return err
}

type AmendTarget struct{ Hash, Message string }

func (s *Service) AmendTarget(ctx context.Context, id string) (AmendTarget, error) {
	result, err := s.Read(ctx, id, "log", "-1", "--format=%H%x00%B", "HEAD")
	if err != nil {
		return AmendTarget{}, err
	}
	hash, message, _ := strings.Cut(string(result.Stdout), "\x00")
	return AmendTarget{hash, strings.TrimRight(message, "\r\n")}, nil
}

func (s *Service) Amend(ctx context.Context, id, head, message string) error {
	if !commitHash.MatchString(head) || strings.TrimSpace(message) == "" {
		return fmt.Errorf("current commit and message are required")
	}
	_, err := s.writeLocked(ctx, id, func() (git.Result, error) {
		revision, err := s.Read(ctx, id, "rev-list", "--parents", "-n", "1", "HEAD")
		if err != nil {
			return git.Result{}, err
		}
		fields := strings.Fields(string(revision.Stdout))
		if len(fields) > 2 || len(fields) == 0 || fields[0] != head {
			return git.Result{}, fmt.Errorf("HEAD changed or is a merge commit")
		}
		shared, err := s.Read(ctx, id, "for-each-ref", "--contains="+head, "--format=%(refname)", "refs/remotes")
		if err != nil {
			return git.Result{}, err
		}
		if len(bytes.TrimSpace(shared.Stdout)) != 0 {
			return git.Result{}, fmt.Errorf("commit is already on a remote-tracking branch")
		}
		for _, marker := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "rebase-merge", "rebase-apply", "sequencer"} {
			if marker == "rebase-merge" || marker == "rebase-apply" || marker == "sequencer" {
				if s.GitStateExists(ctx, id, marker) {
					return git.Result{}, fmt.Errorf("a Git operation is in progress")
				}
			} else if _, err := s.Read(ctx, id, "rev-parse", "--verify", "--quiet", marker); err == nil {
				return git.Result{}, fmt.Errorf("a Git operation is in progress")
			}
		}
		return s.Read(ctx, id, "commit", "--amend", "-m", message)
	})
	return err
}

func (s *Service) CherryPick(ctx context.Context, id, action, hash string, mainline int) error {
	var args []string
	switch action {
	case "start":
		if !commitHash.MatchString(hash) {
			return fmt.Errorf("invalid commit hash")
		}
		args = []string{"cherry-pick"}
		if mainline > 0 {
			args = append(args, fmt.Sprintf("--mainline=%d", mainline))
		}
		args = append(args, "--", hash)
	case "continue", "abort", "skip":
		args = []string{"-c", "core.editor=true", "cherry-pick", "--" + action}
	default:
		return fmt.Errorf("invalid cherry-pick action")
	}
	_, err := s.writeLocked(ctx, id, func() (git.Result, error) {
		inProgress := s.GitStateExists(ctx, id, "CHERRY_PICK_HEAD")
		if action == "start" && inProgress {
			return git.Result{}, fmt.Errorf("a cherry-pick is already in progress")
		}
		if action != "start" && !inProgress {
			return git.Result{}, fmt.Errorf("no cherry-pick in progress")
		}
		return s.Read(ctx, id, args...)
	})
	return err
}

type Worktree struct {
	Path, Branch, Head        string
	Primary, Locked, Prunable bool
}

func (s *Service) Worktrees(ctx context.Context, id string) ([]Worktree, error) {
	result, err := s.Read(ctx, id, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	var out []Worktree
	for field := range strings.SplitSeq(string(result.Stdout), "\x00") {
		key, value, _ := strings.Cut(field, " ")
		if key == "worktree" {
			out = append(out, Worktree{Path: value, Primary: len(out) == 0})
			continue
		}
		if len(out) == 0 {
			continue
		}
		item := &out[len(out)-1]
		switch key {
		case "HEAD":
			item.Head = value
		case "branch":
			item.Branch = strings.TrimPrefix(value, "refs/heads/")
		case "locked":
			item.Locked = true
		case "prunable":
			item.Prunable = true
		}
	}
	return out, nil
}

func (s *Service) EditWorktree(ctx context.Context, id, action, path, branch string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("enter an absolute worktree path")
	}
	path = filepath.Clean(path)
	switch action {
	case "add":
		if branch == "" || strings.HasPrefix(branch, "-") {
			return fmt.Errorf("enter a local branch name")
		}
		ref := "refs/heads/" + branch
		if _, err := s.Read(ctx, id, "show-ref", "--verify", "--", ref); err != nil {
			return err
		}
		_, err := s.Write(ctx, id, "worktree", "add", "--", path, branch)
		return err
	case "remove":
		repo, err := s.Registry.Get(id)
		if err != nil {
			return err
		}
		if repository.Key(path) == repository.Key(repo.Root) {
			return fmt.Errorf("switch to another worktree before removing this one")
		}
		target, err := s.Registry.Discover(ctx, path)
		if err != nil {
			return err
		}
		if target.ID == id {
			return fmt.Errorf("switch to another worktree before removing this one")
		}
		_, err = s.Write(ctx, id, "worktree", "remove", "--", path)
		if err != nil {
			return err
		}
		if _, err := s.Registry.Get(target.ID); err == nil {
			return s.Registry.Unregister(target.ID)
		}
		return nil
	default:
		return fmt.Errorf("invalid worktree action")
	}
}

func (s *Service) RestoreToken(ctx context.Context, id, path string, untracked bool) (string, error) {
	repo, err := s.Registry.Get(id)
	if err != nil {
		return "", err
	}
	path, err = repository.Relative(repo.Root, path)
	if err != nil {
		return "", err
	}
	args := []string{"diff", "--binary", "--no-ext-diff", "--no-textconv", "--no-renames", "--", path}
	if untracked {
		args = []string{"hash-object", "--no-filters", "--", path}
	}
	digest := sha256.New()
	if err := s.Runner.RunTo(ctx, repo.Root, digest, args...); err != nil {
		return "", err
	}
	sum := digest.Sum(nil)
	empty := sha256.Sum256(nil)
	if bytes.Equal(sum, empty[:]) {
		return "", fmt.Errorf("file no longer has eligible changes")
	}
	return fmt.Sprintf("%x", sum), nil
}

func (s *Service) RestoreFile(ctx context.Context, id, path, token string, untracked bool) error {
	repo, err := s.Registry.Get(id)
	if err != nil {
		return err
	}
	path, err = repository.Relative(repo.Root, path)
	if err != nil {
		return err
	}
	_, err = s.writeLocked(ctx, id, func() (git.Result, error) {
		current, err := s.RestoreToken(ctx, id, path, untracked)
		if err != nil {
			return git.Result{}, err
		}
		if token == "" || token != current {
			return git.Result{}, fmt.Errorf("file changed after preview; review it again")
		}
		files, err := s.Status(ctx, id)
		if err != nil {
			return git.Result{}, err
		}
		for _, file := range files {
			if file.Path != path {
				continue
			}
			if untracked && file.Worktree == git.Untracked {
				return s.Read(ctx, id, "clean", "-f", "--", path)
			}
			if !untracked && file.Worktree != git.Clean && file.Worktree != git.Untracked && file.State != git.Conflicted && file.Index != git.Deleted {
				return s.Read(ctx, id, "restore", "--worktree", "--", path)
			}
		}
		return git.Result{}, fmt.Errorf("file no longer has eligible changes")
	})
	return err
}

func (s *Service) GitStateExists(ctx context.Context, id, marker string) bool {
	repo, err := s.Registry.Get(id)
	if err != nil {
		return false
	}
	result, err := s.Read(ctx, id, "rev-parse", "--git-path", marker)
	if err != nil {
		return false
	}
	path := strings.TrimRight(string(result.Stdout), "\r\n")
	if !filepath.IsAbs(path) {
		path = filepath.Join(repo.Root, path)
	}
	_, err = os.Stat(path)
	return err == nil
}
