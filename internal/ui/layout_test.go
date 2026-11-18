package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	native "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"mygittu/internal/app"
	git "mygittu/internal/git"
	"mygittu/internal/repository"
	"mygittu/internal/status"
)

func TestWorkspaceAtNarrow150Percent(t *testing.T) {
	d := New(nil, "", "")
	d.selected = "demo"
	d.repos = []repository.Info{{ID: "demo", Root: "D:/Projects/MyGittu"}}
	d.snapshot = status.Snapshot{Branch: "main", Tracking: app.BranchStatus{Upstream: "origin/main", Compared: true, Ahead: 2}}
	d.changes = []git.FileStatus{
		{Path: "README.md", Index: git.Clean, Worktree: git.Modified, State: git.Modified},
		{Path: "internal/ui/workspace_updates.go", Index: git.Clean, Worktree: git.Modified, State: git.Modified},
		{Path: "internal/app/workbench.go", Index: git.Added, Worktree: git.Clean, State: git.Added},
	}
	d.row, d.stagedCount, d.message = 0, 1, "完善工作区布局"
	d.tools.batch = map[string]bool{"README.md": true, "internal/ui/workspace_updates.go": true}
	d.applyComparison("README.md", app.Comparison{Old: "# MyGittu\n\n打开仓库后开始工作。\n", New: "# MyGittu\n\n打开仓库后开始工作。\n\n可拖动文件区边界调整差异视图的宽度。\n"})

	tt := native.NewTester(d.view, 1280, 800)
	tt.SetSize(960, 680)
	tt.SetScale(1.5)
	if d.comparison.Layout != editor.DiffSplit {
		t.Fatal("default compact workspace should fit side-by-side diff")
	}
	saveLayoutPreview(t, tt, "workspace-compact-960-150.png")
	d.filePane = 380
	tt.Frame()
	tt.Press(452, 318)
	tt.Move(427, 318)
	tt.Release(427, 318)
	tt.Move(20, 20)
	if d.filePane >= 380 {
		t.Fatal("dragging the file divider did not resize the pane")
	}
	if d.comparison.Layout != editor.DiffUnified || tt.Image().Bounds().Dx() != 1440 {
		t.Fatalf("narrow DPI layout not applied: layout=%v size=%v", d.comparison.Layout, tt.Image().Bounds())
	}
	for _, label := range []string{"已选 2 项", "暂存所选", "取消暂存所选", "清空选择"} {
		bounds, ok := tt.Find(label)
		if !ok || bounds.X < 0 || bounds.X+bounds.W > 960 || bounds.Y+bounds.H > 680 {
			t.Fatalf("not fully visible at 150%%: %s %v", label, bounds)
		}
	}
	stage, _ := tt.Find("暂存所选")
	unstage, _ := tt.Find("取消暂存所选")
	clearButton, _ := tt.Find("清空选择")
	if stage.Y != unstage.Y || stage.Y != clearButton.Y {
		t.Fatal("batch buttons should share a baseline at the tested width")
	}
	saveLayoutPreview(t, tt, "workspace-960-150.png")
	if err := tt.Click("清空选择"); err != nil {
		t.Fatal(err)
	}
	if len(d.tools.batch) != 0 {
		t.Fatal("batch selection was not cleared")
	}
	tt.ClickAt(26, 23)
	if !d.sidebarCollapsed {
		t.Fatal("sidebar state did not collapse")
	}
	tt.ClickAt(26, 23)
	if d.sidebarCollapsed {
		t.Fatal("sidebar did not reopen")
	}
	tt.SetSize(1280, 800)
	tt.SetScale(1)
	if d.comparison.Layout != editor.DiffSplit {
		t.Fatal("wider workspace should show side-by-side diff")
	}
	saveLayoutPreview(t, tt, "workspace-1280-100.png")
}

func TestBranchListAtNarrow150Percent(t *testing.T) {
	d := New(nil, "", "")
	d.selected, d.page = "demo", 3
	d.repos = []repository.Info{{ID: "demo", Root: "D:/Projects/MyGittu"}}
	d.tools.branches = []app.BranchEntry{
		{Name: "main", Current: true, Upstream: "origin/main"},
		{Name: "feature/workspace"},
		{Name: "refs/remotes/origin/main", Remote: true},
	}
	d.tools.branchTargets = []string{"main", "feature/workspace", "refs/remotes/origin/main"}
	d.tools.branchSelected = "main"
	d.tools.left, d.tools.right = "main", "feature/workspace"
	tt := native.NewTester(d.view, 960, 680)
	tt.SetScale(1.5)
	if err := tt.Click("feature/workspace"); err != nil {
		t.Fatal(err)
	}
	if d.tools.branchSelected != "feature/workspace" {
		t.Fatal("branch row did not become selected")
	}
	d.tools.compared = true
	d.tools.comparison = app.BranchComparison{Graph: "< c0132fa  修订主分支说明\n| > 6e48d99  支持工作区调整\n|/\no 8a61b32  更新工作区\n", Files: "diff --git a/README.md b/README.md\n+ 支持调节文件列表宽度。\n"}
	tt.Frame()
	for _, label := range []string{"当前分支", "↗ origin/main", "独有提交图", "文件差异"} {
		if !tt.HasText(label) {
			t.Fatalf("missing branch UI: %s", label)
		}
	}
	saveLayoutPreview(t, tt, "branches-960-150.png")
	if err := tt.Click("文件差异"); err != nil {
		t.Fatal(err)
	}
	if d.tools.compareTab != 1 {
		t.Fatal("file differences tab did not open")
	}
}

func TestWorkspaceRepositoryDropdown(t *testing.T) {
	d := New(nil, "", "")
	d.selected = "one"
	d.repos = []repository.Info{
		{ID: "one", Root: "work/MyGittu", LastOpened: 2},
		{ID: "two", Root: "archive/MyGittu", LastOpened: 1},
		{ID: "discovered", Root: "external/MyGittu"},
	}
	tt := native.NewTester(d.view, 960, 680)
	r, ok := tt.Find("MyGittu")
	if !ok {
		t.Fatal("repository dropdown missing")
	}
	tt.Press(r.X+r.W/2, r.Y+r.H/2)
	tt.Release(r.X+r.W/2, r.Y+r.H/2)
	items := tt.Menu()
	if len(items) < 2 || items[0] != "work/MyGittu" || items[1] != "archive/MyGittu" {
		t.Fatalf("same-name repositories are not distinguishable in the dropdown: %v", items)
	}
	if err := tt.ChooseMenuItem(d.text("repo.search")); err != nil {
		t.Fatal(err)
	}
	if !d.repositoriesOpen {
		t.Fatal("repository search did not open from the dropdown")
	}
}

func saveLayoutPreview(t *testing.T, tt *native.Tester, name string) {
	t.Helper()
	dir := os.Getenv("MYGITTU_LAYOUT_PREVIEW")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}
