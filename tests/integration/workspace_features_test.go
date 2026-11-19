package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestProjectFileSavePreservesWorktreeAndRejectsStaleVersion(t *testing.T) {
	s, id, dir := setup(t)
	ctx := context.Background()
	const name = "notes.txt"
	target := filepath.Join(dir, name)
	write(t, target, "before\r\n")
	command(t, dir, "add", name)
	preview, err := s.PreviewFile(ctx, id, name, 0, "")
	if err != nil || preview.New != "before\r\n" {
		t.Fatalf("preview = %+v, %v", preview, err)
	}
	version, err := s.SaveProjectFile(ctx, id, name, preview.Version, []byte("after\r\n"))
	if err != nil || version == "" {
		t.Fatalf("save version = %q, %v", version, err)
	}
	if got := strings.TrimSpace(command(t, dir, "show", ":"+name)); got != "before" {
		t.Fatalf("save changed the Git index: %q", got)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "after\r\n" {
		t.Fatalf("worktree contents = %q, %v", data, err)
	}
	write(t, target, "external edit\r\n")
	if _, err := s.SaveProjectFile(ctx, id, name, version, []byte("replacement")); err == nil {
		t.Fatal("stale save overwrote an external edit")
	}
}

func TestTagVersionOrderAndTimestamps(t *testing.T) {
	service, id, dir := setup(t)
	command(t, dir, "commit", "--allow-empty", "-m", "initial")
	command(t, dir, "tag", "v0.0.9")
	command(t, dir, "tag", "--annotate", "--message=release", "v0.0.19")
	refs, err := service.References(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"v0.0.19", "v0.0.9"}; !slices.Equal(refs.Tags, want) {
		t.Fatalf("unexpected version order: %v", refs.Tags)
	}
	annotated, lightweight := refs.TagTimes["v0.0.19"], refs.TagTimes["v0.0.9"]
	if annotated.Unix == 0 || !annotated.Annotated || lightweight.Unix == 0 || lightweight.Annotated {
		t.Fatalf("unexpected timestamp sources: %+v", refs.TagTimes)
	}
}

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
