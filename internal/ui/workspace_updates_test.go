package ui

import (
	"fmt"
	"path/filepath"
	"testing"

	"mygittu/internal/app"
	git "mygittu/internal/git"
	"mygittu/internal/status"
)

func TestProjectFolderStateSurvivesRepositorySwitchAndRestart(t *testing.T) {
	d := New(nil, "", "")
	d.draftDir = filepath.Join(t.TempDir(), "drafts")
	d.selectRepository("first", "first")
	d.projectExpanded = map[string]bool{"docs": true, "docs/examples": true}
	d.saveProjectFolders()
	d.selectRepository("second", "second")
	if len(d.projectExpanded) != 0 {
		t.Fatal("folder state leaked between repositories")
	}
	d.selectRepository("first", "first")
	if !d.projectExpanded["docs"] || !d.projectExpanded["docs/examples"] {
		t.Fatal("folder state was lost when switching repositories")
	}
	delete(d.projectExpanded, "docs")
	d.saveProjectFolders()
	reopened := New(nil, "", "")
	reopened.draftDir = d.draftDir
	reopened.selectRepository("first", "first")
	if reopened.projectExpanded["docs"] || !reopened.projectExpanded["docs/examples"] {
		t.Fatal("restart did not restore the collapsed parent and expanded child")
	}
}

func TestUpdatedDiffKeepsView(t *testing.T) {
	d := New(nil, "", "")
	d.applyComparison("notes.txt", app.Comparison{Old: "one", New: "two"})
	view := d.comparison
	view.SplitPercent = 42
	if !d.applyComparison("notes.txt", app.Comparison{Old: "one", New: "three"}) ||
		d.comparison != view || view.New.String() != "three" || view.SplitPercent != 42 {
		t.Fatal("updated content replaced the diff view")
	}
}

func TestProjectTreeSearchWithTenThousandFiles(t *testing.T) {
	d := New(nil, "", "")
	for i := range 10000 {
		d.projectFiles = append(d.projectFiles, fmt.Sprintf("src/pkg-%02d/file-%04d.go", i/100, i))
	}
	d.rebuildProjectTree()
	if len(d.projectTree) != 1 || !d.projectTree[0].dir {
		t.Fatal("collapsed tree exposed descendants")
	}
	d.projectSearch = "FILE-9876"
	d.rebuildProjectTree()
	if len(d.projectTree) != 3 || d.projectTree[2].path != "src/pkg-98/file-9876.go" {
		t.Fatalf("incorrect filtered tree: %+v", d.projectTree)
	}
}

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
	file := git.FileStatus{Path: "first/b.txt", Index: git.Clean, State: git.Modified}
	d.changes, d.row = []git.FileStatus{file}, 0
	inserted := git.FileStatus{Path: "last/A.txt", Index: git.Added, State: git.Added}
	root := git.FileStatus{Path: "0.txt", Index: git.Clean, State: git.Modified}
	d.applyWorkspaceStatus(status.Snapshot{Files: []git.FileStatus{root, file, inserted}})
	if d.row != 1 || d.changes[0].Path != inserted.Path || d.changes[2].Path != root.Path || d.message != "ongoing draft" || d.stagedCount != 1 {
		t.Fatal("updated status displaced selection or draft")
	}
	d.applyWorkspaceStatus(status.Snapshot{Files: []git.FileStatus{inserted}})
	if d.row != -1 || d.message != "ongoing draft" {
		t.Fatal("selection remained after the file disappeared")
	}
}
