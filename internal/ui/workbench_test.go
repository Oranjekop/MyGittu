package ui

import (
	"slices"
	"testing"

	native "github.com/egoist/mygo/ui"
	"mygittu/internal/app"
	git "mygittu/internal/git"
)

func TestWorkbenchDialogsRender(t *testing.T) {
	d := New(nil, "", "")
	d.selected = "repo"
	d.tools.conflictOpen = true
	d.tools.conflictPath = "conflicted.txt"
	d.tools.conflict = app.Conflict{Ours: "local", Theirs: "incoming"}
	tester := native.NewTester(d.view, 1160, 740)
	if !tester.HasText(d.text("conflict.save")) || !tester.HasText(d.text("conflict.theirs")) {
		t.Fatal("conflict actions are missing")
	}
	if err := tester.Click(d.text("conflict.theirs")); err != nil {
		t.Fatal(err)
	}
	if d.tools.resolution != "incoming" {
		t.Fatal("selecting the incoming version did not update the result")
	}
	d.tools.conflictOpen = false
	d.confirmRecovery("undo", "example")
	tester.Frame()
	if !tester.HasText(d.text("recover.undo.help")) {
		t.Fatal("undo explanation is missing")
	}
}

func TestBatchPathsIncludesSelectedRenamesOnly(t *testing.T) {
	d := New(nil, "", "")
	d.row = 2
	d.tools.batch = map[string]bool{"new.txt": true, "updated.txt": true}
	files := []git.FileStatus{{Path: "new.txt", OriginalPath: "old.txt"}, {Path: "updated.txt"}, {Path: "preview-only.txt"}}
	if got := d.batchPaths(files); !slices.Equal(got, []string{"new.txt", "old.txt", "updated.txt"}) {
		t.Fatalf("unexpected batch paths: %v", got)
	}
}
