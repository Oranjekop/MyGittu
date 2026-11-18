package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	git "mygittu/internal/git"
	"mygittu/internal/repository"
)

type Conflict struct {
	Ours, Theirs, Working, Token string
}

func (s *Service) Conflict(ctx context.Context, id, path string) (Conflict, error) {
	repo, err := s.Registry.Get(id)
	if err != nil {
		return Conflict{}, err
	}
	path, err = repository.Relative(repo.Root, path)
	if err != nil {
		return Conflict{}, err
	}
	index, err := s.Read(ctx, id, "ls-files", "--unmerged", "-z", "--", path)
	if err != nil {
		return Conflict{}, err
	}
	stages := map[string]string{}
	for record := range bytes.SplitSeq(index.Stdout, []byte{0}) {
		header, name, _ := bytes.Cut(record, []byte{'\t'})
		fields := strings.Fields(string(header))
		if string(name) == path && len(fields) == 3 && (fields[0] == "100644" || fields[0] == "100755") {
			stages[fields[2]] = fields[1]
		}
	}
	if stages["2"] == "" || stages["3"] == "" {
		return Conflict{}, fmt.Errorf("both sides must contain a regular text file")
	}
	working, binary, err := readWorktreeFile(repo.Root, path)
	if err != nil {
		return Conflict{}, err
	}
	ours, err := s.Read(ctx, id, "cat-file", "blob", stages["2"])
	if err != nil {
		return Conflict{}, err
	}
	theirs, err := s.Read(ctx, id, "cat-file", "blob", stages["3"])
	if err != nil {
		return Conflict{}, err
	}
	for _, data := range [][]byte{working, ours.Stdout, theirs.Stdout} {
		if binary || len(data) > 256<<10 || !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			return Conflict{}, fmt.Errorf("conflict editor requires text files up to 256 KiB")
		}
	}
	hash := sha256.New()
	hash.Write(index.Stdout)
	hash.Write(working)
	return Conflict{Ours: string(ours.Stdout), Theirs: string(theirs.Stdout), Working: string(working), Token: fmt.Sprintf("%x", hash.Sum(nil))}, nil
}

func (s *Service) ResolveConflict(ctx context.Context, id, path, token, content string) error {
	if len(content) > 256<<10 || strings.ContainsRune(content, 0) {
		return fmt.Errorf("invalid resolution text")
	}
	for line := range strings.SplitSeq(content, "\n") {
		if strings.HasPrefix(line, "<<<<<<< ") || strings.HasPrefix(line, ">>>>>>> ") || strings.HasPrefix(line, "||||||| ") {
			return fmt.Errorf("remove the remaining conflict markers before saving")
		}
	}
	_, err := s.writeLocked(ctx, id, func() (git.Result, error) {
		current, err := s.Conflict(ctx, id, path)
		if err != nil {
			return git.Result{}, err
		}
		if current.Token != token {
			return git.Result{}, fmt.Errorf("conflict changed; reload it before saving")
		}
		repo, err := s.Registry.Get(id)
		if err != nil {
			return git.Result{}, err
		}
		path, err = repository.Relative(repo.Root, path)
		if err != nil {
			return git.Result{}, err
		}
		root, err := os.OpenRoot(repo.Root)
		if err != nil {
			return git.Result{}, err
		}
		defer root.Close()
		info, err := root.Lstat(path)
		if err != nil {
			return git.Result{}, err
		}
		if !info.Mode().IsRegular() {
			return git.Result{}, fmt.Errorf("resolution target is not a regular file")
		}
		tmp := filepath.Join(filepath.Dir(path), ".mygittu-resolve-"+token[:24])
		file, err := root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			return git.Result{}, err
		}
		defer root.Remove(tmp)
		_, err = file.WriteString(content)
		closeErr := file.Close()
		if err != nil {
			return git.Result{}, err
		}
		if closeErr != nil {
			return git.Result{}, closeErr
		}
		if err := root.Rename(tmp, path); err != nil {
			return git.Result{}, err
		}
		result, err := s.Read(ctx, id, "add", "--", path)
		if err != nil {
			return result, fmt.Errorf("resolution saved but not staged: %w", err)
		}
		return result, nil
	})
	return err
}

func (s *Service) FileInsight(ctx context.Context, id, path, action, hash string) (string, error) {
	repo, err := s.Registry.Get(id)
	if err != nil {
		return "", err
	}
	path, err = repository.Relative(repo.Root, path)
	if err != nil {
		return "", err
	}
	var args []string
	switch action {
	case "blame":
		args = []string{"blame", "-l", "--root", "--date=short", "--", path}
	case "patch":
		if !commitHash.MatchString(hash) {
			return "", fmt.Errorf("invalid commit hash")
		}
		history, err := s.Read(ctx, id, "log", "--follow", "--format=%x00%H", "--name-only", "-z", "--", path)
		if err != nil {
			return "", err
		}
		records := bytes.Split(history.Stdout, []byte{0})
		for i := 1; i+1 < len(records); i++ {
			if len(records[i-1]) == 0 && string(records[i]) == hash && bytes.HasPrefix(records[i+1], []byte{'\n'}) {
				path = string(records[i+1][1:])
				break
			}
		}
		args = []string{"show", "--no-ext-diff", "--no-textconv", "--no-color", hash, "--", path}
	default:
		return "", fmt.Errorf("unknown file insight")
	}
	result, err := s.Read(ctx, id, args...)
	return string(result.Stdout), err
}

func (s *Service) RecoverCommit(ctx context.Context, id, action, hash string) error {
	if action == "merge.continue" {
		_, err := s.writeLocked(ctx, id, func() (git.Result, error) {
			if _, err := s.Read(ctx, id, "rev-parse", "--verify", "MERGE_HEAD"); err != nil {
				return git.Result{}, fmt.Errorf("no merge in progress")
			}
			return s.Read(ctx, id, "commit", "--no-edit")
		})
		return err
	}
	if !commitHash.MatchString(hash) {
		return fmt.Errorf("invalid commit hash")
	}
	if action == "revert" {
		_, err := s.Write(ctx, id, "revert", "--no-edit", "--", hash)
		return err
	}
	if action != "undo" {
		return fmt.Errorf("unknown recovery action")
	}
	_, err := s.writeLocked(ctx, id, func() (git.Result, error) {
		head, err := s.Read(ctx, id, "rev-list", "--parents", "-n", "1", "HEAD")
		if err != nil {
			return git.Result{}, err
		}
		fields := strings.Fields(string(head.Stdout))
		if len(fields) != 2 || fields[0] != hash {
			return git.Result{}, fmt.Errorf("select the current single-parent commit")
		}
		shared, err := s.Read(ctx, id, "for-each-ref", "--contains="+hash, "--format=%(refname)", "refs/remotes")
		if err != nil {
			return git.Result{}, err
		}
		if len(bytes.TrimSpace(shared.Stdout)) != 0 {
			return git.Result{}, fmt.Errorf("commit is already present on a remote-tracking branch; use revert")
		}
		return s.Read(ctx, id, "reset", "--soft", fields[1])
	})
	return err
}

type BranchComparison struct{ Graph, Files string }

type BranchEntry struct {
	Name, Upstream  string
	Current, Remote bool
}

func (s *Service) BranchEntries(ctx context.Context, id string) ([]BranchEntry, error) {
	result, err := s.Read(ctx, id, "for-each-ref", "--format=%(refname)%09%(HEAD)%09%(upstream:short)%09%(symref)", "refs/heads", "refs/remotes")
	if err != nil {
		return nil, err
	}
	entries := make([]BranchEntry, 0)
	for line := range strings.SplitSeq(string(result.Stdout), "\n") {
		parts := strings.Split(strings.TrimSuffix(line, "\r"), "\t")
		if len(parts) != 4 || parts[0] == "" || parts[3] != "" {
			continue
		}
		entries = append(entries, BranchEntry{
			Name:     strings.TrimPrefix(parts[0], "refs/heads/"),
			Upstream: parts[2], Current: parts[1] == "*", Remote: strings.HasPrefix(parts[0], "refs/remotes/"),
		})
	}
	return entries, nil
}

func (s *Service) CompareTargets(ctx context.Context, id string) ([]string, error) {
	result, err := s.Read(ctx, id, "for-each-ref", "--format=%(refname) %(symref)", "refs/heads", "refs/remotes")
	if err != nil {
		return nil, err
	}
	var names []string
	for line := range strings.SplitSeq(string(result.Stdout), "\n") {
		ref, target, _ := strings.Cut(line, " ")
		if ref != "" && strings.TrimSpace(target) == "" {
			names = append(names, strings.TrimPrefix(ref, "refs/heads/"))
		}
	}
	return names, nil
}

func (s *Service) CompareBranches(ctx context.Context, id, left, right string) (BranchComparison, error) {
	var comparison BranchComparison
	resolve := func(name string) (string, error) {
		if name == "" || strings.HasPrefix(name, "-") {
			return "", fmt.Errorf("branch name required")
		}
		ref := "refs/heads/" + name
		if strings.HasPrefix(name, "refs/remotes/") {
			ref = name
		}
		if _, err := s.Read(ctx, id, "check-ref-format", ref); err != nil {
			return "", err
		}
		result, err := s.Read(ctx, id, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
		return strings.TrimSpace(string(result.Stdout)), err
	}
	a, err := resolve(left)
	if err != nil {
		return comparison, err
	}
	b, err := resolve(right)
	if err != nil {
		return comparison, err
	}
	result, err := s.Read(ctx, id, "log", "--graph", "--left-right", "--boundary", "--oneline", "--no-color", "--max-count=200", a+"..."+b, "--")
	if err != nil {
		return comparison, err
	}
	comparison.Graph = string(result.Stdout)
	result, err = s.Read(ctx, id, "diff", "--no-ext-diff", "--no-textconv", "--no-color", a, b, "--")
	comparison.Files = string(result.Stdout)
	return comparison, err
}
