package ui

import (
	"errors"
	"slices"
	"testing"

	native "github.com/egoist/mygo/ui"
	"mygittu/internal/app"
	git "mygittu/internal/git"
	"mygittu/internal/repository"
)

func TestPartiallyStagedFileAppearsInBothGroups(t *testing.T) {
	files := []git.FileStatus{
		{Path: "partial.txt", Index: git.Modified, Worktree: git.Modified, State: git.Modified},
		{Path: "staged.txt", Index: git.Added, Worktree: git.Clean, State: git.Added},
		{Path: "new.txt", Index: git.Clean, Worktree: git.Untracked, State: git.Untracked},
		{Path: "conflict.txt", Index: git.Conflicted, Worktree: git.Conflicted, State: git.Conflicted},
	}
	want := []workspaceChangeRow{
		{file: -1, count: 2, staged: true}, {file: 0, staged: true}, {file: 1, staged: true},
		{file: -1, count: 3}, {file: 0}, {file: 2}, {file: 3},
	}
	if got := workspaceChangeRows(files, false, false); !slices.Equal(got, want) {
		t.Fatalf("incorrect group rows: %+v", got)
	}
	d := New(nil, "", "")
	d.selected = "demo"
	d.repos = []repository.Info{{ID: "demo", Root: "/tmp/demo"}}
	d.changes, d.stagedCount = files, 2
	tt := native.NewTester(d.view, 960, 680)
	tt.SetScale(1.5)
	if tt.HasText("staged.txt") || tt.HasText("partial.txt") {
		t.Fatal("both groups should initially be collapsed")
	}
	for _, label := range []string{"已暂存的更改（2）", "更改（3）"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if err := tt.Click("已暂存的更改（2）"); err != nil {
		t.Fatal(err)
	}
	if !d.stagedCollapsed || tt.HasText("staged.txt") || !tt.HasText("partial.txt") {
		t.Fatal("collapsing staged changes should leave the working-tree copy visible")
	}
	if err := tt.Click("已暂存的更改（2）"); err != nil {
		t.Fatal(err)
	}
	if d.stagedCollapsed || !tt.HasText("staged.txt") {
		t.Fatal("staged changes did not reopen")
	}
	saveLayoutPreview(t, tt, "grouped-changes-150.png")
}

func TestSelectAllChangesIncludesCollapsedGroups(t *testing.T) {
	d := New(nil, "", "")
	d.selected = "demo"
	d.changes = []git.FileStatus{
		{Path: "staged.go", Index: git.Added, Worktree: git.Clean},
		{Path: "working.go", Index: git.Clean, Worktree: git.Modified},
	}
	tt := native.NewTester(d.view, 960, 680)
	if err := tt.Click("全选"); err != nil {
		t.Fatal(err)
	}
	stage, unstage, count := d.stagingSelection(d.changes)
	if count != 2 || !slices.Equal(stage, []string{"working.go"}) || !slices.Equal(unstage, []string{"staged.go"}) {
		t.Fatal("select all did not feed staged and unstaged files into the existing batch actions")
	}
	if err := tt.Click(d.text("batch.clear")); err != nil {
		t.Fatal(err)
	}
	if len(d.tools.batch) != 0 {
		t.Fatal("clear selection did not undo select all")
	}
}

func TestFileTabBarArrowsWhenTabsOverflow(t *testing.T) {
	d := New(nil, "", "")
	var bar fileTabBarState
	width := float32(80)
	tt := native.NewTester(func(c *native.Context) {
		d.fileTabBar(c, &bar, func() { native.Box(c).Width(width).Height(24) })
	}, 320, 100)
	left, right := "向左滚动标签", "向右滚动标签"
	if tt.HasText(right) {
		t.Fatal("arrows should be hidden when tabs fit")
	}
	width = 750
	tt.Frame()
	tt.Frame()
	if err := tt.Click(right); err != nil {
		t.Fatal(err)
	}
	if bar.scroll.X <= 0 {
		t.Fatal("right arrow did not scroll the tabs")
	}
	if err := tt.Click(left); err != nil {
		t.Fatal(err)
	}
	if bar.scroll.X != 0 {
		t.Fatal("left arrow did not return to the start")
	}
	width = 80
	tt.Frame()
	tt.Frame()
	if tt.HasText(right) || bar.overflow {
		t.Fatal("arrows stayed after the tabs fit again")
	}
}

func TestChangeTabsOpenSwitchAndClose(t *testing.T) {
	d := New(nil, "", "")
	d.selected = "demo"
	d.changes = []git.FileStatus{
		{Path: "src/first.go", Index: git.Modified, Worktree: git.Modified},
		{Path: "docs/second.md", Worktree: git.Modified},
	}
	d.activateChangeTab(changeTab{path: "src/first.go", staged: true})
	d.activateChangeTab(changeTab{path: "docs/second.md"})
	tt := native.NewTester(d.showChangeTabs, 700, 100)
	if err := tt.Click("first.go"); err != nil {
		t.Fatal(err)
	}
	if d.row != 0 || !d.staged || len(d.changeTabs) != 2 {
		t.Fatal("switching tabs did not restore the staged side")
	}
	if err := tt.Click("关闭 src/first.go"); err != nil {
		t.Fatal(err)
	}
	if d.row != 1 || d.staged || len(d.changeTabs) != 1 {
		t.Fatal("closing the active tab did not select its neighbor")
	}
	if err := tt.Click("关闭 docs/second.md"); err != nil {
		t.Fatal(err)
	}
	if d.row != -1 || len(d.changeTabs) != 0 {
		t.Fatal("closing the last tab did not reset the preview")
	}
}

func TestRetryPushAndUpdateControlsAreScoped(t *testing.T) {
	d := New(nil, "", "")
	d.selected = "demo"
	d.repos = []repository.Info{{ID: "demo", Root: "/tmp/demo"}}
	d.recordCommitPush("demo", errors.New("remote rejected push"))
	d.update = app.UpdateInfo{Available: true, Latest: "9.0.0"}
	tt := native.NewTester(d.view, 960, 680)
	if !tt.HasText("重试推送") || tt.HasText("下载并安装 9.0.0") {
		t.Fatal("the workspace should show push recovery and keep download in settings")
	}
	saveLayoutPreview(t, tt, "push-retry-and-update-marker.png")
	if err := tt.Click("MyGittu 9.0.0 可用"); err != nil {
		t.Fatal(err)
	}
	if !d.settingsOpen || d.settingsTab != 3 || !tt.HasText("下载并安装 9.0.0") {
		t.Fatal("update marker did not open Software Updates")
	}
	saveLayoutPreview(t, tt, "update-settings-from-marker.png")
	d.recordCommitPush("demo", nil)
	d.settingsOpen = false
	tt.Frame()
	before, ok := tt.Find(d.text("diff.empty"))
	if !ok {
		t.Fatal("missing workspace empty state")
	}
	d.remoteNotice = "当前分支已是最新"
	tt.Frame()
	after, ok := tt.Find(d.text("diff.empty"))
	if !ok || after.Y != before.Y || !tt.HasText(d.remoteNotice) {
		t.Fatal("the toolbar notice should not move the workspace")
	}
	saveLayoutPreview(t, tt, "toolbar-no-layout-shift.png")
	d.remoteRunning, d.busy = "pull", true
	tt.Frame()
	if tt.HasText("重试推送") {
		t.Fatal("successful retry should clear the failure action")
	}
	if !tt.HasText(d.text("remote.pull")) || tt.HasText("拉取中…") {
		t.Fatal("pull progress should keep the toolbar button label stable")
	}
	saveLayoutPreview(t, tt, "inline-remote-progress.png")
}

func TestWorkspaceDiffSideFollowsFileGroup(t *testing.T) {
	partial := git.FileStatus{Index: git.Modified, Worktree: git.Modified}
	if !workspaceDiffSide(partial, true) || workspaceDiffSide(partial, false) {
		t.Fatal("a partially staged file should preserve the selected side")
	}
	if !workspaceDiffSide(git.FileStatus{Index: git.Modified, Worktree: git.Clean}, false) {
		t.Fatal("a fully staged file should move to the staged side")
	}
	if workspaceDiffSide(git.FileStatus{Index: git.Clean, Worktree: git.Modified}, true) {
		t.Fatal("an unstaged file should move to the working-tree side")
	}
}

func TestEmptyChangesHidesGroupsWhilePulling(t *testing.T) {
	d := New(nil, "", "")
	d.selected = "demo"
	d.repos = []repository.Info{{ID: "demo", Root: "/tmp/demo"}}
	tt := native.NewTester(d.view, 960, 680)
	tt.SetScale(1.5)
	if !tt.HasText("暂无变更") {
		t.Fatal("expected the empty workspace illustration")
	}
	empty, _ := tt.Find("暂无变更")
	background := tt.Image().RGBAAt(750, 450)
	d.busy, d.remoteRunning = true, "pull"
	tt.Frame()
	if !tt.HasText(d.text("remote.pull")) || tt.HasText("0 个变更") || tt.HasText("全选") || tt.HasText(d.text("stage.all")) {
		t.Fatal("empty changes should not expose bulk actions while pulling")
	}
	if after, visible := tt.Find("暂无变更"); !visible || after != empty || tt.Image().RGBAAt(750, 450) != background {
		t.Fatal("pull should keep the underlying workspace visible without a scrim")
	}
	saveLayoutPreview(t, tt, "empty-changes-while-pulling.png")
}
