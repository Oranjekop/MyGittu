package status

import (
	"context"
	"fmt"
	"mygittu/internal/app"
	git "mygittu/internal/git"
	"mygittu/internal/repository"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func fixture(tb testing.TB) (*Cache, repository.Info) {
	tb.Helper()
	dir := tb.TempDir()
	if out, e := exec.Command("git", "init", dir).CombinedOutput(); e != nil {
		tb.Fatalf("git init: %s %v", out, e)
	}
	r, e := git.NewRunner("")
	if e != nil {
		tb.Fatal(e)
	}
	reg, e := repository.New(r, "")
	if e != nil {
		tb.Fatal(e)
	}
	info, e := reg.Register(context.Background(), dir)
	if e != nil {
		tb.Fatal(e)
	}
	return New(&app.Service{Runner: r, Registry: reg}), info
}
func TestRepositoryOverlayTracksFetchedChanges(t *testing.T) {
	c, repo := fixture(t)
	ctx := context.Background()
	run := func(dir string, args ...string) {
		t.Helper()
		if _, err := c.Service.Runner.Run(ctx, dir, args...); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(dir string) {
		run(dir, "-c", "user.name=Test", "-c", "user.email=test@example.invalid",
			"-c", "commit.gpgsign=false", "commit", "-m", "Update")
	}
	file := filepath.Join(repo.Root, "tracked.txt")
	if err := os.WriteFile(file, []byte("original\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run(repo.Root, "add", "tracked.txt")
	commit(repo.Root)
	run(repo.Root, "branch", "-M", "main")
	upstream := t.TempDir()
	run(upstream, "clone", repo.Root, ".")
	run(repo.Root, "remote", "add", "upstream", upstream)
	run(repo.Root, "fetch", "upstream")
	run(repo.Root, "branch", "--set-upstream-to=upstream/main")
	check := func(want git.State) {
		t.Helper()
		if err := c.Refresh(ctx, repo.ID); err != nil {
			t.Fatal(err)
		}
		if got := c.Lookup(repo.Root).State; got != want {
			t.Fatalf("repository overlay = %s, want %s", got, want)
		}
		if got := c.Lookup(file).State; got != git.Clean {
			t.Fatalf("tracked file overlay = %s, want clean", got)
		}
	}
	check(git.Clean)
	if err := os.WriteFile(filepath.Join(upstream, "tracked.txt"), []byte("updated\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run(upstream, "add", "tracked.txt")
	commit(upstream)
	if fetched, err := c.Service.FetchUpstream(ctx, repo.ID); err != nil || !fetched {
		t.Fatalf("background fetch: fetched=%t, error=%v", fetched, err)
	}
	check(git.Modified)
	run(repo.Root, "merge", "--ff-only", "upstream/main")
	check(git.Clean)
}

func TestWatcherRefreshAndStop(t *testing.T) {
	c, repo := fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("watcher failed to stop")
		}
	}()
	await := func(state git.State) {
		t.Helper()
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			if c.Lookup(repo.Root).State == state {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("expected %s, got %+v", state, c.Lookup(repo.Root))
	}
	await(git.Clean)
	initial := c.Lookup(repo.Root)
	c.ScheduleRefresh(repo.ID)
	deadline := time.Now().Add(8 * time.Second)
	for {
		got := c.Lookup(repo.Root)
		if got.State != git.Clean {
			t.Fatalf("scheduled refresh hid the clean overlay: %+v", got)
		}
		if got.Generation > initial.Generation {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("scheduled refresh was not processed without a filesystem event")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if e := os.WriteFile(filepath.Join(repo.Root, "new.txt"), []byte("new"), 0600); e != nil {
		t.Fatal(e)
	}
	await(git.Untracked)
}
func TestLookupBoundaries(t *testing.T) {
	c, repo := fixture(t)
	c.snapshots[repo.ID] = Snapshot{Generation: 1, states: map[string]git.State{".": git.Clean, repository.Key("ignored"): git.Ignored, repository.Key(filepath.Join("dir", "file")): git.Conflicted, repository.Key("dir"): git.Conflicted}}
	for _, p := range []string{filepath.Join(repo.Root, "..", "outside"), filepath.Join(repo.Root, ".git", "config"), "relative"} {
		if got := c.Lookup(p); got.State != git.Unknown {
			t.Errorf("unexpected overlay for %q: %+v", p, got)
		}
	}
	if c.Lookup(filepath.Join(repo.Root, "ignored", "child")).State != git.Ignored {
		t.Fatal("ignored directory propagation")
	}
	if c.Lookup(filepath.Join(repo.Root, "dir")).State != git.Conflicted {
		t.Fatal("conflict directory")
	}
}

func TestDirtyRepositoryKeepsOverlayUntilRefresh(t *testing.T) {
	c, repo := fixture(t)
	ctx := context.Background()
	if err := c.Refresh(ctx, repo.ID); err != nil {
		t.Fatal(err)
	}
	before := c.Lookup(repo.Root)
	c.ScheduleRefresh(repo.ID)
	c.ScheduleRefresh(repo.ID)
	if queued := c.takeScheduled(); len(queued) != 1 {
		t.Fatalf("expected one scheduled refresh, got %v", queued)
	}
	if got := c.Lookup(repo.Root); got != before || !got.Cached {
		t.Fatalf("badge disappeared while refresh was pending: %+v", got)
	}
}
func BenchmarkLookup100K(b *testing.B) {
	c, repo := fixture(b)
	states := make(map[string]git.State, 100001)
	states["."] = git.Modified
	for i := 0; i < 100000; i++ {
		states[repository.Key(fmt.Sprintf("file-%06d", i))] = git.Modified
	}
	c.snapshots[repo.ID] = Snapshot{Generation: 1, states: states}
	path := filepath.Join(repo.Root, "file-050000")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.Lookup(path)
	}
}
