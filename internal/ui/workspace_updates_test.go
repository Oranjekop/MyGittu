package ui

import (
	"testing"

	git "mygittu/internal/git"
	"mygittu/internal/status"
)

func TestCommitDraftsAreIsolatedAndPersisted(t *testing.T) {
	dir := t.TempDir()
	d := New(nil, "", "")
	d.draftDir = dir
	d.selectRepository("first", "first")
	d.message = "first draft"
	d.selectRepository("second", "second")
	if d.message != "" {
		t.Fatal("draft leaked into another repository")
	}
	d.message = "second draft"
	d.saveDraft()
	reopened := New(nil, "", "")
	reopened.draftDir = dir
	reopened.selectRepository("first", "first")
	if reopened.message != "first draft" {
		t.Fatal("first draft did not survive restart")
	}
	reopened.message = ""
	reopened.saveDraft()
	reopened.selectRepository("second", "second")
	if reopened.message != "second draft" {
		t.Fatal("clearing the first draft changed the second")
	}
}

func TestWorkspaceStatusPreservesSelectionByPath(t *testing.T) {
	d := New(nil, "", "")
	d.message = "ongoing draft"
	file := git.FileStatus{Path: "b.txt", Index: git.Clean, State: git.Modified}
	d.changes, d.row = []git.FileStatus{file}, 0
	inserted := git.FileStatus{Path: "a.txt", Index: git.Added, State: git.Added}
	d.applyWorkspaceStatus(status.Snapshot{Files: []git.FileStatus{inserted, file}})
	if d.row != 1 || d.message != "ongoing draft" || d.stagedCount != 1 {
		t.Fatal("updated status displaced selection or draft")
	}
	d.applyWorkspaceStatus(status.Snapshot{Files: []git.FileStatus{inserted}})
	if d.row != -1 || d.message != "ongoing draft" {
		t.Fatal("selection remained after the file disappeared")
	}
}
