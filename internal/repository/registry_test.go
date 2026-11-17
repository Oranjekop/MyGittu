package repository

import (
	"context"
	git "mygittu/internal/git"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRegistryPersistence(t *testing.T) {
	root := t.TempDir()
	if out, e := exec.Command("git", "init", root).CombinedOutput(); e != nil {
		t.Fatalf("init: %s %v", out, e)
	}
	r, e := git.NewRunner("")
	if e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(t.TempDir(), "repositories.json")
	reg, e := New(r, file)
	if e != nil {
		t.Fatal(e)
	}
	a, e := reg.Register(context.Background(), root)
	if e != nil {
		t.Fatal(e)
	}
	b, e := reg.Register(context.Background(), root)
	if e != nil || a.ID != b.ID || len(reg.List()) != 1 {
		t.Fatal("registration not idempotent")
	}
	reload, e := New(r, file)
	if e != nil || len(reload.List()) != 1 {
		t.Fatalf("reload: %v", e)
	}
	if reload.List()[0].LastOpened != b.LastOpened {
		t.Fatal("recent order timestamp not persisted")
	}
	if e = reload.Unregister(a.ID); e != nil {
		t.Fatal(e)
	}
	empty, e := New(r, file)
	if e != nil || len(empty.List()) != 0 {
		t.Fatal("unregister not persisted")
	}
}

func TestFindGitRootFromUnopenedChild(t *testing.T) {
	root := t.TempDir()
	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %s %v", out, err)
	}
	child := filepath.Join(root, "src", "pkg")
	if err := os.MkdirAll(child, 0700); err != nil {
		t.Fatal(err)
	}
	got, ok := FindGitRoot(filepath.Join(child, "main.go"))
	if !ok || filepath.Clean(got) != filepath.Clean(root) {
		t.Fatalf("FindGitRoot=%q,%v want %q,true", got, ok, root)
	}
}
