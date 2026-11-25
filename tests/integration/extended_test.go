package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mygittu/internal/app"
	"mygittu/internal/status"
)

func TestAdvancedRepositoryOperations(t *testing.T) {
	s, id, dir := setup(t)
	ctx := context.Background()
	if entries, err := s.Reflog(ctx, id); err != nil || len(entries) != 0 {
		t.Fatalf("empty reflog: %+v, %v", entries, err)
	}
	path := filepath.Join(dir, "a.txt")
	write(t, path, "base\n")
	command(t, dir, "add", ".")
	command(t, dir, "commit", "-m", "base")
	base := strings.TrimSpace(command(t, dir, "rev-parse", "HEAD"))
	write(t, path, "fixed\n")
	patch, err := s.ExportPatch(ctx, id, false)
	if err != nil || patch == "" {
		t.Fatalf("export: %v", err)
	}
	command(t, dir, "restore", "a.txt")
	check, err := s.ApplyPatch(ctx, id, patch, "")
	if err != nil || check.Token == "" || check.Summary == "" {
		t.Fatalf("check: %+v, %v", check, err)
	}
	if _, err := s.ApplyPatch(ctx, id, patch, "wrong"); err == nil {
		t.Fatal("applied patch with wrong token")
	}
	if _, err := s.ApplyPatch(ctx, id, patch, check.Token); err != nil {
		t.Fatal(err)
	}
	command(t, dir, "commit", "-am", "fixed")
	fixed := strings.TrimSpace(command(t, dir, "rev-parse", "HEAD"))
	write(t, filepath.Join(dir, "b.txt"), "independent\n")
	command(t, dir, "add", "b.txt")
	command(t, dir, "commit", "-m", "independent")
	head := strings.TrimSpace(command(t, dir, "rev-parse", "HEAD"))
	entries, err := s.Reflog(ctx, id)
	if err != nil || len(entries) < 3 || entries[0].Hash != head {
		t.Fatalf("reflog: %+v, %v", entries, err)
	}
	if err := s.BranchAt(ctx, id, "recovered", entries[1].Hash); err != nil {
		t.Fatal(err)
	}
	if output, err := s.Bisect(ctx, id, "start", head, base); err != nil || output == "" {
		t.Fatalf("bisect start: %q, %v", output, err)
	}
	if output, err := s.Bisect(ctx, id, "bad", "", ""); err != nil || !strings.Contains(output, fixed) {
		t.Fatalf("bisect bad: %q, %v", output, err)
	}
	if _, err := s.Bisect(ctx, id, "reset", "", ""); err != nil {
		t.Fatal(err)
	}
	steps, err := s.RebasePlan(ctx, id, base)
	if err != nil || len(steps) != 2 {
		t.Fatalf("plan: %+v, %v", steps, err)
	}
	reordered := []app.RebaseStep{steps[1], {Hash: steps[0].Hash, Action: "fixup"}}
	if err := s.Rebase(ctx, id, "start", base, reordered); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(command(t, dir, "log", "--format=%s", base+"..HEAD")); got != "independent" {
		t.Fatalf("unexpected rewritten log: %q", got)
	}
	if got := strings.TrimSpace(command(t, dir, "show", "HEAD:a.txt")); got != "fixed" {
		t.Fatalf("fixup lost file change: %q", got)
	}
}

func TestRemotesAndFirstPush(t *testing.T) {
	s, id, dir := setup(t)
	ctx := context.Background()
	remote := t.TempDir()
	command(t, remote, "init", "--bare", "-b", "main")
	write(t, filepath.Join(dir, "a.txt"), "base\n")
	command(t, dir, "add", "a.txt")
	command(t, dir, "commit", "-m", "base")
	if err := s.ConfigureRemote(ctx, id, "add", "origin", remote); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfigureRemote(ctx, id, "push", "origin", "main"); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(command(t, dir, "rev-parse", "--abbrev-ref", "@{upstream}")); got != "origin/main" {
		t.Fatalf("upstream=%q", got)
	}
	command(t, dir, "branch", "--unset-upstream")
	if err := s.ConfigureRemote(ctx, id, "track", "origin", "main"); err != nil {
		t.Fatal(err)
	}
	peer := filepath.Join(t.TempDir(), "peer")
	command(t, dir, "clone", remote, peer)
	command(t, peer, "config", "user.name", "Peer")
	command(t, peer, "config", "user.email", "peer@example.invalid")
	command(t, peer, "config", "commit.gpgsign", "false")
	write(t, filepath.Join(peer, "a.txt"), "remote\n")
	command(t, peer, "commit", "-am", "remote")
	command(t, peer, "push")
	if err := s.ConfigureRemote(ctx, id, "pull", "origin", "main"); err != nil {
		t.Fatal(err)
	}
	if got := command(t, dir, "show", "HEAD:a.txt"); strings.TrimSpace(got) != "remote" {
		t.Fatal(got)
	}
	next := t.TempDir()
	if err := s.ConfigureRemote(ctx, id, "set-url", "origin", next); err != nil {
		t.Fatal(err)
	}
	remotes, err := s.Remotes(ctx, id)
	if err != nil || len(remotes) != 1 || filepath.Clean(remotes[0].URL) != filepath.Clean(next) {
		t.Fatalf("remotes=%+v err=%v", remotes, err)
	}
	if err := s.ConfigureRemote(ctx, id, "remove", "origin", ""); err != nil {
		t.Fatal(err)
	}
}

func TestAmendPreservesBodyAndRejectsPublishedCommit(t *testing.T) {
	s, id, dir := setup(t)
	ctx := context.Background()
	command(t, dir, "commit", "--allow-empty", "-m", "base", "-m", "long description")
	target, err := s.AmendTarget(ctx, id)
	if err != nil || !strings.Contains(target.Message, "long description") {
		t.Fatalf("target=%+v err=%v", target, err)
	}
	write(t, filepath.Join(dir, "extra.txt"), "staged\n")
	command(t, dir, "add", "extra.txt")
	if err := s.Amend(ctx, id, target.Hash, target.Message); err != nil {
		t.Fatal(err)
	}
	amended, err := s.AmendTarget(ctx, id)
	if err != nil || amended.Hash == target.Hash || amended.Message != target.Message {
		t.Fatalf("amended=%+v err=%v", amended, err)
	}
	command(t, dir, "show", "HEAD:extra.txt")
	command(t, dir, "update-ref", "refs/remotes/origin/main", amended.Hash)
	if err := s.Amend(ctx, id, amended.Hash, "shared"); err == nil {
		t.Fatal("published commit was amended")
	}
}

func TestCherryPickConflictContinueAndAbort(t *testing.T) {
	s, id, dir := setup(t)
	ctx := context.Background()
	file := filepath.Join(dir, "conflict.txt")
	write(t, file, "base\n")
	command(t, dir, "add", ".")
	command(t, dir, "commit", "-m", "base")
	command(t, dir, "switch", "-c", "feature")
	write(t, file, "feature\n")
	command(t, dir, "commit", "-am", "feature")
	hash := strings.TrimSpace(command(t, dir, "rev-parse", "HEAD"))
	command(t, dir, "switch", "main")
	write(t, file, "main\n")
	command(t, dir, "commit", "-am", "main")
	for _, action := range []string{"abort", "skip", "continue"} {
		if err := s.CherryPick(ctx, id, "start", hash, 0); err == nil {
			t.Fatal("expected conflict")
		}
		if err := s.CherryPick(ctx, id, "start", hash, 0); err == nil || !strings.Contains(err.Error(), "already in progress") {
			t.Fatalf("expected the active cherry-pick to block another start, got %v", err)
		}
		cache := status.New(s)
		if err := cache.Refresh(ctx, id); err != nil {
			t.Fatal(err)
		}
		snapshot, ok := cache.Get(id)
		if !ok || !snapshot.CherryPicking {
			t.Fatal("cherry-pick state missing")
		}
		if action == "continue" {
			write(t, file, "resolved\n")
			command(t, dir, "add", "conflict.txt")
		}
		if err := s.CherryPick(ctx, id, action, "", 0); err != nil {
			t.Fatal(err)
		}
		if s.GitStateExists(ctx, id, "CHERRY_PICK_HEAD") {
			t.Fatal("state not cleared")
		}
	}
	if got := strings.TrimSpace(command(t, dir, "show", "HEAD:conflict.txt")); got != "resolved" {
		t.Fatal(got)
	}
}

func TestCherryPickMergeRelativeToFirstParent(t *testing.T) {
	s, id, dir := setup(t)
	command(t, dir, "commit", "--allow-empty", "-m", "base")
	command(t, dir, "branch", "target")
	command(t, dir, "switch", "-c", "feature")
	write(t, filepath.Join(dir, "feature.txt"), "feature\n")
	command(t, dir, "add", "feature.txt")
	command(t, dir, "commit", "-m", "feature")
	command(t, dir, "switch", "main")
	command(t, dir, "merge", "--no-ff", "-m", "merge feature", "feature")
	hash := strings.TrimSpace(command(t, dir, "rev-parse", "HEAD"))
	command(t, dir, "switch", "target")
	if err := s.CherryPick(context.Background(), id, "start", hash, 1); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(command(t, dir, "show", "HEAD:feature.txt")); got != "feature" {
		t.Fatal(got)
	}
}

func TestRestoreFileAndHunk(t *testing.T) {
	s, id, dir := setup(t)
	ctx := context.Background()
	command(t, dir, "config", "core.autocrlf", "false")
	file := filepath.Join(dir, "lines.txt")
	var lines []string
	for i := range 32 {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	save := func() { write(t, file, strings.Join(lines, "\n")+"\n") }
	save()
	command(t, dir, "add", ".")
	command(t, dir, "commit", "-m", "base")
	lines[2] = "staged"
	save()
	command(t, dir, "add", "lines.txt")
	lines[4], lines[24] = "first hunk", "second hunk"
	save()
	hunks, err := s.StageHunks(ctx, id, "lines.txt")
	if err != nil || len(hunks) != 2 {
		t.Fatalf("hunks=%+v err=%v", hunks, err)
	}
	if err := s.RestoreSelectedHunk(ctx, id, "lines.txt", hunks[0].Token, 0); err != nil {
		t.Fatal(err)
	}
	diff := command(t, dir, "diff")
	if strings.Contains(diff, "first hunk") || !strings.Contains(diff, "second hunk") {
		t.Fatal(diff)
	}
	token, err := s.RestoreToken(ctx, id, "lines.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RestoreFile(ctx, id, "lines.txt", "stale", false); err == nil {
		t.Fatal("stale restore allowed")
	}
	if err := s.RestoreFile(ctx, id, "lines.txt", token, false); err != nil {
		t.Fatal(err)
	}
	worktree, err := s.Read(ctx, id, "diff")
	if err != nil || len(worktree.Stdout) != 0 {
		t.Fatalf("remaining worktree diff: %s (%v)", worktree.Stdout, err)
	}
	index, err := s.Read(ctx, id, "diff", "--cached")
	if err != nil || !strings.Contains(string(index.Stdout), "+staged") {
		t.Fatalf("staged content was not preserved: %s (%v)", index.Stdout, err)
	}
	loose := filepath.Join(dir, "new.txt")
	write(t, loose, "untracked\n")
	token, err = s.RestoreToken(ctx, id, "new.txt", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RestoreFile(ctx, id, "new.txt", token, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(loose); !os.IsNotExist(err) {
		t.Fatal("untracked file still exists", err)
	}
}

func TestWorktreeLifecycle(t *testing.T) {
	s, id, dir := setup(t)
	ctx := context.Background()
	command(t, dir, "commit", "--allow-empty", "-m", "base")
	command(t, dir, "branch", "feature")
	path := filepath.Join(t.TempDir(), "feature tree")
	if err := s.EditWorktree(ctx, id, "add", path, "feature"); err != nil {
		t.Fatal(err)
	}
	rows, err := s.Worktrees(ctx, id)
	if err != nil || len(rows) != 2 || !rows[0].Primary || rows[1].Branch != "feature" {
		t.Fatalf("trees=%+v err=%v", rows, err)
	}
	linked, err := s.Registry.Register(ctx, path)
	if err != nil || linked.ID == id {
		t.Fatalf("linked=%+v err=%v", linked, err)
	}
	write(t, filepath.Join(path, "new.txt"), "keep\n")
	if err := s.EditWorktree(ctx, id, "remove", path, ""); err == nil {
		t.Fatal("dirty worktree was removed")
	}
	if err := os.Remove(filepath.Join(path, "new.txt")); err != nil {
		t.Fatal(err)
	}
	if err := s.EditWorktree(ctx, id, "remove", path, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Registry.Get(linked.ID); err == nil {
		t.Fatal("removed worktree remains registered")
	}
}
