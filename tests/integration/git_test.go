package integration

import (
	"context"
	"mygittu/internal/app"
	"mygittu/internal/commitai"
	git "mygittu/internal/git"
	"mygittu/internal/repository"
	"mygittu/internal/status"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func command(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	b, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v %s", args, e, b)
	}
	return string(b)
}
func setup(t *testing.T) (*app.Service, string, string) {
	t.Helper()
	dir := t.TempDir()
	command(t, dir, "init", "-b", "main")
	command(t, dir, "config", "user.name", "Test")
	command(t, dir, "config", "user.email", "test@example.invalid")
	command(t, dir, "config", "commit.gpgsign", "false")
	r, e := git.NewRunner("")
	if e != nil {
		t.Fatal(e)
	}
	reg, e := repository.New(r, filepath.Join(t.TempDir(), "repos.json"))
	if e != nil {
		t.Fatal(e)
	}
	repo, e := reg.Register(context.Background(), dir)
	if e != nil {
		t.Fatal(e)
	}
	return &app.Service{Runner: r, Registry: reg}, repo.ID, dir
}
func TestStagedCommitContextScalesWithDiffSize(t *testing.T) {
	s, id, dir := setup(t)
	ctx := context.Background()
	command(t, dir, "config", "core.autocrlf", "false")
	write(t, filepath.Join(dir, "z-last.txt"), "later staged change\n")
	write(t, filepath.Join(dir, "unstaged.txt"), "unstaged content must stay local\n")
	command(t, dir, "add", "z-last.txt")
	small, err := s.StagedCommitContext(ctx, id)
	if err != nil || !strings.HasPrefix(small, "diff --git ") || !strings.Contains(small, "+later staged change") {
		t.Fatalf("expected complete small diff: %v", err)
	}
	for _, scenario := range []struct {
		size     int
		excerpts bool
	}{
		{128 << 10, true},
		{9 << 20, false},
	} {
		write(t, filepath.Join(dir, "a-large.txt"), strings.Repeat("x", scenario.size)+"\n")
		command(t, dir, "add", "a-large.txt")
		got, err := s.StagedCommitContext(ctx, id)
		if err != nil || len(got) > commitai.MaxPatch || !strings.Contains(got, "2 files changed") ||
			!strings.Contains(got, "a-large.txt") || !strings.Contains(got, "z-last.txt") ||
			strings.Contains(got, "unstaged") || strings.Contains(got, "+later staged change") != scenario.excerpts {
			t.Fatalf("unexpected context for %d-byte change: %d bytes, error %v", scenario.size, len(got), err)
		}
	}
}

func TestConflictStatus(t *testing.T) {
	s, id, dir := setup(t)
	ctx := context.Background()
	file := filepath.Join(dir, "conflict.txt")
	write(t, file, "base\n")
	command(t, dir, "add", ".")
	command(t, dir, "commit", "-m", "base")
	command(t, dir, "switch", "-c", "feature")
	write(t, file, "feature\n")
	command(t, dir, "commit", "-am", "feature")
	command(t, dir, "switch", "main")
	write(t, file, "main\n")
	command(t, dir, "commit", "-am", "main")
	if e := s.Branch(ctx, id, "merge", "feature"); e == nil {
		t.Fatal("merge should conflict")
	}
	files, e := s.Status(ctx, id)
	if e != nil || len(files) != 1 || files[0].State != git.Conflicted {
		t.Fatalf("conflict: %+v %v", files, e)
	}
	cache := status.New(s)
	if e = cache.Refresh(ctx, id); e != nil {
		t.Fatal(e)
	}
	if cache.Lookup(dir).State != git.Conflicted {
		t.Fatal("conflict aggregation lost")
	}
}
func TestLocalRemoteWorkflow(t *testing.T) {
	s, id, dir := setup(t)
	ctx := context.Background()
	remote := filepath.Join(t.TempDir(), "origin.git")
	command(t, t.TempDir(), "init", "--bare", remote)
	write(t, filepath.Join(dir, "a"), "a")
	command(t, dir, "add", "a")
	command(t, dir, "commit", "-m", "base")
	command(t, dir, "remote", "add", "origin", remote)
	command(t, dir, "push", "-u", "origin", "main")
	write(t, filepath.Join(dir, "a"), "b")
	command(t, dir, "commit", "-am", "next")
	pushed, err := s.RemoteWithResult(ctx, id, "push")
	if err != nil || pushed.NothingToPush {
		t.Fatalf("push with a new commit: %+v, %v", pushed, err)
	}
	unchanged, err := s.RemoteWithResult(ctx, id, "push")
	if err != nil || !unchanged.NothingToPush {
		t.Fatalf("up-to-date push: %+v, %v", unchanged, err)
	}
	command(t, dir, "update-ref", "-d", "refs/remotes/origin/main")
	fetched, err := s.RemoteWithResult(ctx, id, "fetch")
	if err != nil || fetched.UpToDate {
		t.Fatalf("fetch restoring a tracking ref: %+v, %v", fetched, err)
	}
	for _, action := range []string{"fetch", "pull"} {
		result, err := s.RemoteWithResult(ctx, id, action)
		if err != nil || !result.UpToDate {
			t.Fatalf("up-to-date %s: %+v, %v", action, result, err)
		}
	}
}
func write(t *testing.T, path, content string) {
	t.Helper()
	if e := os.WriteFile(path, []byte(content), 0600); e != nil {
		t.Fatal(e)
	}
}

func TestCommitDetailsIncludesRootAndMergePatches(t *testing.T) {
	s, id, dir := setup(t)
	ctx := context.Background()
	write(t, filepath.Join(dir, "root.txt"), "initial\n")
	command(t, dir, "add", ".")
	command(t, dir, "commit", "-m", "Root commit")
	root := strings.TrimSpace(command(t, dir, "rev-parse", "HEAD"))
	details, err := s.CommitDetails(ctx, id, root)
	if err != nil || !strings.Contains(details.Summary, "Root commit") || !strings.Contains(details.Patch, "+initial") {
		t.Fatalf("root details: %+v, %v", details, err)
	}
	command(t, dir, "switch", "-c", "feature")
	write(t, filepath.Join(dir, "feature.txt"), "feature\n")
	command(t, dir, "add", ".")
	command(t, dir, "commit", "-m", "Feature commit")
	command(t, dir, "switch", "main")
	command(t, dir, "merge", "--no-ff", "-m", "Merge feature", "feature")
	merge := strings.TrimSpace(command(t, dir, "rev-parse", "HEAD"))
	details, err = s.CommitDetails(ctx, id, merge)
	if err != nil || !strings.Contains(details.Summary, "Merge feature") || !strings.Contains(details.Patch, "+feature") {
		t.Fatalf("merge details: %+v, %v", details, err)
	}
}
func TestGitWorkflow(t *testing.T) {
	s, id, dir := setup(t)
	ctx := context.Background()
	name := "中文 [literal].txt"
	write(t, filepath.Join(dir, name), "first\n")
	write(t, filepath.Join(dir, "second.txt"), "other\n")
	files, e := s.Status(ctx, id)
	if e != nil || len(files) != 2 {
		t.Fatalf("status: %v %+v", e, files)
	}
	if e = s.Paths(ctx, id, []string{name}, true); e != nil {
		t.Fatal(e)
	}
	if e = s.Paths(ctx, id, []string{name}, false); e != nil {
		t.Fatal("unborn unstage", e)
	}
	if e = s.Paths(ctx, id, []string{name}, true); e != nil {
		t.Fatal(e)
	}
	hash, e := s.Commit(ctx, id, "initial commit\n\nbody")
	if e != nil || len(hash) != 40 {
		t.Fatalf("commit %q %v", hash, e)
	}
	write(t, filepath.Join(dir, name), "first\nsecond\n")
	if e = s.Paths(ctx, id, []string{name}, true); e != nil {
		t.Fatal(e)
	}
	write(t, filepath.Join(dir, name), "first\nsecond\nthird\n")
	files, e = s.Status(ctx, id)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, f := range files {
		if f.Path == name {
			found = f.Index == git.Modified && f.Worktree == git.Modified
		}
	}
	if !found {
		t.Fatalf("partial stage lost: %+v", files)
	}
	staged, e := s.Diff(ctx, id, name, true)
	if e != nil || staged == "" {
		t.Fatalf("diff: %v %q", e, staged)
	}
	unstaged, e := s.Diff(ctx, id, name, false)
	if e != nil || unstaged == staged {
		t.Fatal("index/worktree diff not distinct")
	}
	if e = s.Paths(ctx, id, []string{name}, false); e != nil {
		t.Fatal(e)
	}
	logs, e := s.Log(ctx, id, 10, 0)
	if e != nil || len(logs) != 1 || logs[0].Hash != hash || logs[0].Subject != "initial commit" {
		t.Fatalf("log: %v %+v", e, logs)
	}
	cache := status.New(s)
	if e = cache.Refresh(ctx, id); e != nil {
		t.Fatal(e)
	}
	if got := cache.Lookup(filepath.Join(dir, name)); got.State != git.Modified || !got.Cached {
		t.Fatalf("overlay: %+v, dir=%q repos=%+v", got, dir, s.Registry.List())
	}
	if cache.Lookup(dir).State != git.Modified {
		t.Fatal("directory aggregation")
	}
	cache.Invalidate(id)
	if cache.Lookup(dir).State != git.Unknown {
		t.Fatal("stale overlay")
	}
	for _, p := range []string{"../outside", filepath.Join(dir, "absolute"), "."} {
		if e = s.Paths(ctx, id, []string{p}, true); e == nil {
			t.Errorf("accepted unsafe path %q", p)
		}
	}
	if e = s.Branch(ctx, id, "create", "feature"); e != nil {
		t.Fatal(e)
	}
	if e = s.Branch(ctx, id, "switch", "feature"); e != nil {
		t.Fatal(e)
	}
	if e = s.Branch(ctx, id, "switch", "main"); e != nil {
		t.Fatal(e)
	}
	if e = s.Branch(ctx, id, "delete", "feature"); e != nil {
		t.Fatal(e)
	}
}
func TestRenameAndDelete(t *testing.T) {
	s, id, dir := setup(t)
	ctx := context.Background()
	write(t, filepath.Join(dir, "old name"), "content\n")
	if e := s.Paths(ctx, id, []string{"old name"}, true); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Commit(ctx, id, "base"); e != nil {
		t.Fatal(e)
	}
	command(t, dir, "mv", "old name", "new name")
	files, e := s.Status(ctx, id)
	if e != nil || len(files) != 1 || files[0].OriginalPath != "old name" {
		t.Fatalf("rename: %v %+v", e, files)
	}
	if e = s.Paths(ctx, id, []string{"new name", "old name"}, false); e != nil {
		t.Fatal("unstage rename", e)
	}
	if e = os.Remove(filepath.Join(dir, "new name")); e != nil {
		t.Fatal(e)
	}
	files, e = s.Status(ctx, id)
	if e != nil || len(files) != 1 || files[0].Worktree != git.Deleted {
		t.Fatalf("delete: %v %+v", e, files)
	}
}
func TestWorktreeRegistry(t *testing.T) {
	s, _, dir := setup(t)
	ctx := context.Background()
	write(t, filepath.Join(dir, "a"), "a")
	command(t, dir, "add", "a")
	command(t, dir, "commit", "-m", "base")
	wt := filepath.Join(t.TempDir(), "worktree")
	command(t, dir, "worktree", "add", "-b", "other", wt)
	info, e := s.Registry.Register(ctx, wt)
	if e != nil || info.Root == dir || info.GitDir == filepath.Join(wt, ".git") {
		t.Fatalf("worktree discovery: %+v %v", info, e)
	}
}
func TestRunnerCancellation(t *testing.T) {
	s, id, _ := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	_, e := s.Read(ctx, id, "status")
	if e == nil || time.Since(start) > time.Second {
		t.Fatal("cancellation failed")
	}
}
