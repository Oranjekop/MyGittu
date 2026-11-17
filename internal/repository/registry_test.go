package repository

import (
	"context"
	git "mygittu/internal/git"
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
