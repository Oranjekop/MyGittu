package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileComparison(t *testing.T) {
	s, id, dir := setup(t)
	ctx := context.Background()
	path := filepath.Join(dir, "file.txt")
	write(t, path, "first\n")
	got, err := s.Comparison(ctx, id, "file.txt", "", false)
	if err != nil || got.Old != "" || got.New != "first\n" {
		t.Fatalf("untracked: %+v %v", got, err)
	}
	command(t, dir, "add", "file.txt")
	got, err = s.Comparison(ctx, id, "file.txt", "", true)
	if err != nil || got.Old != "" || got.New != "first\n" {
		t.Fatalf("unborn staged: %+v %v", got, err)
	}
	command(t, dir, "commit", "-m", "base")
	write(t, path, "second\n")
	command(t, dir, "add", "file.txt")
	write(t, path, "third\n")
	got, err = s.Comparison(ctx, id, "file.txt", "", true)
	if err != nil || got.Old != "first\n" || got.New != "second\n" {
		t.Fatalf("staged: %+v %v", got, err)
	}
	got, err = s.Comparison(ctx, id, "file.txt", "", false)
	if err != nil || got.Old != "second\n" || got.New != "third\n" {
		t.Fatalf("worktree: %+v %v", got, err)
	}
	command(t, dir, "mv", "file.txt", "renamed.txt")
	got, err = s.Comparison(ctx, id, "renamed.txt", "file.txt", true)
	if err != nil || got.Old != "first\n" || got.New != "second\n" {
		t.Fatalf("rename: %+v %v", got, err)
	}
	if err = os.Remove(filepath.Join(dir, "renamed.txt")); err != nil {
		t.Fatal(err)
	}
	got, err = s.Comparison(ctx, id, "renamed.txt", "", false)
	if err != nil || got.New != "" {
		t.Fatalf("delete: %+v %v", got, err)
	}
	write(t, filepath.Join(dir, "binary"), "\x00\x01")
	got, err = s.Comparison(ctx, id, "binary", "", false)
	if err != nil || !got.Binary {
		t.Fatalf("binary: %+v %v", got, err)
	}
	if _, err = s.Comparison(ctx, id, "../escape", "", false); err == nil {
		t.Fatal("path escape allowed")
	}
}

func TestCloneRepository(t *testing.T) {
	s, _, dir := setup(t)
	write(t, filepath.Join(dir, "file"), "base")
	command(t, dir, "add", ".")
	command(t, dir, "commit", "-m", "base")
	dest := filepath.Join(t.TempDir(), "clone")
	repo, err := s.Clone(context.Background(), dir, dest)
	if err != nil || repo.Root == "" {
		t.Fatalf("clone: %+v %v", repo, err)
	}
	if _, err = s.Clone(context.Background(), "ext::untrusted", filepath.Join(t.TempDir(), "bad")); err == nil {
		t.Fatal("external command transport accepted")
	}
}
