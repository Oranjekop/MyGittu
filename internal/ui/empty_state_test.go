package ui

import (
	"testing"

	native "github.com/egoist/mygo/ui"
	"mygittu/internal/settings"
)

func TestWorkspaceEmptyStatesFollowData(t *testing.T) {
	d := New(nil, "", "")
	d.applyPrefs(settings.Values{Language: "zh-CN", Theme: "light"})
	d.selected = "demo"
	d.projectFiles = []string{".github/workflows/release.yml", "LICENSE", "README.md", "assets/icon.png"}
	d.rebuildProjectTree()
	for _, page := range []struct {
		id      int
		label   string
		preview string
	}{
		{0, d.text("diff.empty"), "empty-diff.png"},
		{projectFilesPage, d.text("files.select"), "empty-preview.png"},
		{1, d.text("history.empty"), "empty-history.png"},
		{3, "选择分支进行对比", "empty-branches.png"},
		{referencesPage, "暂无标签", "empty-references.png"},
	} {
		t.Run(page.preview, func(t *testing.T) {
			d.page = page.id
			tester := native.NewTester(d.view, 1160, 740)
			if !tester.HasText(page.label) {
				t.Fatalf("missing empty-state label %q", page.label)
			}
			if page.id == 0 && tester.HasText(d.text("diff.all")) {
				t.Fatal("the empty difference pane should not show its content heading")
			}
			saveLayoutPreview(t, tester, page.preview)
			d.busy = true
			tester.Frame()
			if tester.HasText(page.label) {
				t.Fatalf("empty-state label %q shown during loading", page.label)
			}
			d.busy = false
		})
	}
}
