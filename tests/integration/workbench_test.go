package integration

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkbenchConflictAndGraph(t *testing.T) {
	s, id, dir := setup(t)
	ctx := context.Background()
	path := filepath.Join(dir, "file.txt")
	write(t, path, "base\n")
	command(t, dir, "add", ".")
	command(t, dir, "commit", "-m", "base")
	command(t, dir, "switch", "-c", "feature")
	write(t, path, "incoming\n")
	command(t, dir, "commit", "-am", "feature change")
	command(t, dir, "switch", "main")
	write(t, path, "current\n")
	command(t, dir, "commit", "-am", "main change")
	command(t, dir, "remote", "add", "origin", dir)
	command(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")
	command(t, dir, "branch", "--set-upstream-to=origin/main", "main")
	entries, err := s.BranchEntries(ctx, id)
	if err != nil || len(entries) != 3 || !entries[1].Current || entries[1].Upstream != "origin/main" || !entries[2].Remote {
		t.Fatalf("unexpected branch list: %+v, %v", entries, err)
	}
	comparison, err := s.CompareBranches(ctx, id, "main", "feature")
	if err != nil || !strings.Contains(comparison.Graph, "feature change") || !strings.Contains(comparison.Graph, "main change") || !strings.Contains(comparison.Files, "+incoming") {
		t.Fatalf("branch comparison: %+v, %v", comparison, err)
	}
	if err := s.Branch(ctx, id, "merge", "feature"); err == nil {
		t.Fatal("expected merge conflict")
	}
	conflict, err := s.Conflict(ctx, id, "file.txt")
	if err != nil || !strings.Contains(conflict.Ours, "current") || !strings.Contains(conflict.Theirs, "incoming") {
		t.Fatalf("conflict: %+v %v", conflict, err)
	}
	if err := s.ResolveConflict(ctx, id, "file.txt", conflict.Token, conflict.Working); err == nil {
		t.Fatal("saved unresolved markers")
	}
	write(t, path, "external edit\n")
	if err := s.ResolveConflict(ctx, id, "file.txt", conflict.Token, "merged\n"); err == nil {
		t.Fatal("overwrote external edit")
	}
	conflict, err = s.Conflict(ctx, id, "file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ResolveConflict(ctx, id, "file.txt", conflict.Token, conflict.Theirs); err != nil {
		t.Fatal(err)
	}
	if got := command(t, dir, "diff", "--name-only", "--diff-filter=U"); got != "" {
		t.Fatalf("unresolved: %s", got)
	}
	if err := s.RecoverCommit(ctx, id, "merge.continue", ""); err != nil {
		t.Fatal(err)
	}
	if parents := strings.Fields(command(t, dir, "rev-list", "--parents", "-n1", "HEAD")); len(parents) != 3 {
		t.Fatalf("merge parents: %v", parents)
	}
}

func TestWorkbenchFileHistoryRecoveryAndBatch(t *testing.T) {
	s, id, dir := setup(t)
	ctx := context.Background()
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		write(t, filepath.Join(dir, name), "base\n")
	}
	command(t, dir, "add", ".")
	command(t, dir, "commit", "-m", "base")
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		write(t, filepath.Join(dir, name), "change\n")
	}
	if err := s.Paths(ctx, id, []string{"a.txt", "b.txt"}, true); err != nil {
		t.Fatal(err)
	}
	if got := strings.Fields(command(t, dir, "diff", "--cached", "--name-only")); strings.Join(got, ",") != "a.txt,b.txt" {
		t.Fatalf("unexpected stage: %v", got)
	}
	if err := s.Paths(ctx, id, []string{"a.txt", "b.txt"}, false); err != nil {
		t.Fatal(err)
	}
	if got := command(t, dir, "diff", "--cached", "--name-only"); got != "" {
		t.Fatalf("unexpected index: %s", got)
	}
	command(t, dir, "add", "a.txt")
	command(t, dir, "commit", "-m", "change a")
	log, err := s.FileLog(ctx, id, "a.txt", 100, 0)
	if err != nil || len(log) != 2 {
		t.Fatalf("file history: %+v %v", log, err)
	}
	hash := log[0].Hash
	patch, err := s.FileInsight(ctx, id, "a.txt", "patch", hash)
	if err != nil || !strings.Contains(patch, "+change") {
		t.Fatalf("patch: %s %v", patch, err)
	}
	blame, err := s.FileInsight(ctx, id, "a.txt", "blame", "")
	if err != nil || !strings.Contains(blame, "Test") || !strings.Contains(blame, "change") {
		t.Fatalf("blame: %s %v", blame, err)
	}
	lines, err := s.FileBlame(ctx, id, "a.txt")
	if err != nil || len(lines) != 1 || lines[0].Number != 1 || lines[0].Hash != hash || lines[0].Author != "Test" || lines[0].Text != "change" || lines[0].Date == "" {
		t.Fatalf("structured blame: %+v %v", lines, err)
	}
	if err := s.RecoverCommit(ctx, id, "undo", log[1].Hash); err == nil {
		t.Fatal("accepted stale HEAD")
	}
	if err := s.RecoverCommit(ctx, id, "undo", hash); err != nil {
		t.Fatal(err)
	}
	if staged := command(t, dir, "diff", "--cached", "--name-only"); strings.TrimSpace(staged) != "a.txt" {
		t.Fatalf("undo lost staged content: %s", staged)
	}
	command(t, dir, "commit", "-m", "change a again")
	head := strings.TrimSpace(command(t, dir, "rev-parse", "HEAD"))
	command(t, dir, "update-ref", "refs/remotes/origin/main", head)
	if err := s.RecoverCommit(ctx, id, "undo", head); err == nil {
		t.Fatal("allowed undo of remote-tracked commit")
	}
	if err := s.RecoverCommit(ctx, id, "revert", head); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(command(t, dir, "show", "HEAD:a.txt")); got != "base" {
		t.Fatalf("revert content: %s", got)
	}
	command(t, dir, "mv", "a.txt", "renamed.txt")
	command(t, dir, "commit", "-m", "rename file")
	history, err := s.FileLog(ctx, id, "renamed.txt", 100, 0)
	if err != nil || len(history) < 2 {
		t.Fatalf("renamed history: %v %v", history, err)
	}
	patch, err = s.FileInsight(ctx, id, "renamed.txt", "patch", history[len(history)-1].Hash)
	if err != nil || !strings.Contains(patch, "+base") {
		t.Fatalf("pre-rename patch: %s %v", patch, err)
	}
}
