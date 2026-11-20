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
	before, ok := tt.Find("全部差异")
	if !ok {
		t.Fatal("missing workspace heading")
	}
	d.remoteNotice = "当前分支已是最新"
	tt.Frame()
	after, ok := tt.Find("全部差异")
	if !ok || after.Y != before.Y || !tt.HasText(d.remoteNotice) {
		t.Fatal("the toolbar notice should not move the workspace")
	}
	saveLayoutPreview(t, tt, "toolbar-no-layout-shift.png")
	d.remoteRunning, d.busy = "pull", true
	tt.Frame()
	if tt.HasText("重试推送") {
		t.Fatal("successful retry should clear the failure action")
	}
	if !tt.HasText("拉取中…") {
		t.Fatal("pull progress should appear in its toolbar button")
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
	d.busy, d.remoteRunning = true, "pull"
	tt.Frame()
	if !tt.HasText("拉取中…") || tt.HasText("已暂存的更改（0）") || tt.HasText("更改（0）") {
		t.Fatal("empty groups should stay hidden while pulling")
	}
	saveLayoutPreview(t, tt, "empty-changes-while-pulling.png")
}
