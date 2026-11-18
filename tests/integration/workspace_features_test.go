package integration

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestPartialStageAndReferences(t *testing.T) {
	s, id, dir := setup(t)
	ctx := context.Background()
	path := filepath.Join(dir, "sample.txt")
	var original strings.Builder
	for i := range 40 {
		fmt.Fprintf(&original, "line %d\n", i)
	}
	write(t, path, original.String())
	command(t, dir, "add", ".")
	command(t, dir, "commit", "-m", "baseline")
	command(t, dir, "branch", "upstream")
	command(t, dir, "branch", "--set-upstream-to", "upstream")
	revised := strings.NewReplacer("line 2\n", "first change\n", "line 35\n", "last change\n").Replace(original.String())
	write(t, path, revised)
	hunks, err := s.StageHunks(ctx, id, "sample.txt")
	if err != nil || len(hunks) != 2 {
		t.Fatalf("wanted two hunks: %v %v", hunks, err)
	}
	if err := s.StageSelectedHunk(ctx, id, "sample.txt", hunks[0].Token, 0); err != nil {
		t.Fatal(err)
	}
	staged := command(t, dir, "diff", "--cached")
	if !strings.Contains(staged, "first change") || strings.Contains(staged, "last change") {
		t.Fatalf("incorrect staged content: %s", staged)
	}
	if err := s.StageSelectedHunk(ctx, id, "sample.txt", hunks[0].Token, 0); err == nil {
		t.Fatal("accepted a stale hunk")
	}
	if err := s.EditReference(ctx, id, "stash.save", "pending edit"); err != nil {
		t.Fatal(err)
	}
	if err := s.EditReference(ctx, id, "tag.create", "test-release"); err != nil {
		t.Fatal(err)
	}
	refs, err := s.References(ctx, id)
	if err != nil || len(refs.Stashes) != 1 || !slices.Contains(refs.Tags, "test-release") {
		t.Fatalf("unexpected references: %+v %v", refs, err)
	}
	stashID, _, _ := strings.Cut(refs.Stashes[0], "\t")
	if err := s.EditReference(ctx, id, "stash.apply", stashID); err != nil {
		t.Fatal(err)
	}
	if got := command(t, dir, "diff"); !strings.Contains(got, "last change") {
		t.Fatalf("stash was not restored: %s", got)
	}
	command(t, dir, "commit", "-am", "ahead")
	_, branch, err := s.WorkspaceStatus(ctx, id)
	if err != nil || branch.Upstream != "upstream" || branch.Ahead != 1 || branch.Behind != 0 {
		t.Fatalf("unexpected upstream status: %+v %v", branch, err)
	}
	if err := s.EditReference(ctx, id, "tag.delete", "test-release"); err != nil {
		t.Fatal(err)
	}
}
