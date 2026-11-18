package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	native "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"mygittu/internal/app"
	"mygittu/internal/buildinfo"
	git "mygittu/internal/git"
	"mygittu/internal/repository"
	"mygittu/internal/settings"
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
	d.tools.comparison = app.BranchComparison{Commits: []app.BranchComparisonCommit{
		{Side: "<", Commit: app.Commit{Subject: "修订主分支说明"}},
		{Side: ">", Commit: app.Commit{Subject: "支持工作区调整"}},
	}}
	tt.Frame()
	for _, label := range []string{"当前分支", "↗ origin/main", "独有提交", "支持工作区调整", "文件差异"} {
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

func TestSettingsTabsAndCommitAIAt150Percent(t *testing.T) {
	d := New(nil, "", "")
	d.selected = "demo"
	d.repos = []repository.Info{{ID: "demo", Root: "D:/Projects/MyGittu"}}
	d.changes = []git.FileStatus{{Path: "cmd/desktop/main.go", Index: git.Modified, Worktree: git.Clean, State: git.Modified}}
	d.row, d.stagedCount, d.staged = 0, 1, true
	d.openSettings()
	tt := native.NewTester(d.view, 960, 680)
	tt.SetScale(1.5)
	if !tt.HasText("字体大小") {
		t.Fatal("appearance settings missing")
	}
	saveLayoutPreview(t, tt, "settings-appearance-150.png")
	if err := tt.Click("AI 供应商"); err != nil {
		t.Fatal(err)
	}
	if d.settingsTab != 2 || !tt.HasText("供应商") {
		t.Fatal("AI tab did not open")
	}
	if err := tt.Click("未启用"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("DeepSeek"); err != nil {
		t.Fatal(err)
	}
	if d.aiForm.config.Provider != "DeepSeek" || d.aiForm.config.BaseURL == "" || !tt.HasText("模型 ID") {
		t.Fatal("provider selection did not populate its configuration")
	}
	if !tt.HasText("获取模型 ID") {
		t.Fatal("model retrieval button missing")
	}
	saveLayoutPreview(t, tt, "settings-ai-150.png")
	if err := tt.Click("AI 提示词"); err != nil {
		t.Fatal(err)
	}
	if d.settingsTab != 3 || !tt.HasText("提交信息提示词") || d.aiPromptDraft == "" {
		t.Fatal("AI prompt tab did not open with its default template")
	}
	saveLayoutPreview(t, tt, "settings-ai-prompt-150.png")
	tt.Key(0, native.KeyEscape)
	tt.Frame()
	if d.settingsOpen {
		t.Fatal("settings did not close")
	}
	if err := tt.Click("AI 生成提交信息"); err != nil {
		t.Fatal(err)
	}
	if !d.settingsOpen || d.settingsTab != 2 {
		t.Fatal("unconfigured AI should open provider settings")
	}
}

func TestIntegrationSettingsAt150Percent(t *testing.T) {
	d := New(nil, "", "")
	d.openSettings()
	d.settingsTab = 5
	d.integrationLoaded = true
	d.integration = integrationState{Menu: true, Modern: true, Overlays: true}
	tt := native.NewTester(d.view, 960, 680)
	tt.SetScale(1.5)
	for _, label := range []string{"系统集成", "登录时启动 MyGittu", "当前版本  " + buildinfo.Version} {
		if !tt.HasText(label) {
			t.Fatalf("missing integration setting %q", label)
		}
	}
	if explorerSupported {
		for _, label := range []string{"资源管理器右键菜单", "Git 状态徽标", "重启资源管理器以应用"} {
			if !tt.HasText(label) {
				t.Fatalf("missing Explorer setting %q", label)
			}
		}
	}
	saveLayoutPreview(t, tt, "settings-integration-150.png")
}

func TestExtendedGitDialogsAt150Percent(t *testing.T) {
	d := New(nil, "", "")
	d.selected = "demo"
	d.page = 3
	tt := native.NewTester(d.view, 960, 680)
	tt.SetScale(1.5)
	if !tt.HasText("远程仓库") || !tt.HasText("工作树") {
		t.Fatal("branch tools missing")
	}
	w := &d.repositoryTools
	w.remoteOpen = true
	w.remotes = []app.RemoteEntry{{Name: "origin", URL: "https://example.invalid/team/project.git"}}
	w.name, w.branch = "origin", "main"
	tt.Frame()
	if !tt.HasText("推送并跟踪") || !tt.HasText("设置跟踪") {
		t.Fatal("remote actions missing")
	}
	saveLayoutPreview(t, tt, "remotes-150.png")
	if err := tt.Click("移除远程"); err != nil {
		t.Fatal(err)
	}
	if !w.confirmOpen || !tt.HasText("移除此远程仓库配置？") {
		t.Fatal("remote removal confirmation missing")
	}
	w.confirmOpen = false
	w.remoteOpen = false
	w.treeOpen = true
	w.trees = []app.Worktree{{Path: "D:/Projects/Main", Branch: "main", Primary: true}}
	tt.Frame()
	if !tt.HasText("创建工作树") {
		t.Fatal("worktree actions missing")
	}
	saveLayoutPreview(t, tt, "worktrees-150.png")
	w.treeOpen = false
	w.amendOpen = true
	w.amendMessage = "subject\n\ndescription"
	tt.Frame()
	if !tt.HasText("确认修正") {
		t.Fatal("amend confirmation missing")
	}
}

func TestCloneCredentialProfilesAt150Percent(t *testing.T) {
	d := New(nil, "", "clone")
	d.prefs.CloneProfiles = []settings.CloneProfile{
		{ID: "personal", Label: "GitHub Personal", Kind: "github", Username: "personal-user"},
		{ID: "work", Label: "GitHub Work", Kind: "github", Username: "work-user"},
		{ID: "gitee", Label: "Gitee Work", Kind: "https", Host: "gitee.com", Username: "gitee-user"},
	}
	d.cloneProvider = 1
	tt := native.NewTester(d.view, 960, 680)
	tt.SetScale(1.5)
	if err := tt.Click("Git 默认凭据"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("GitHub Personal") || !tt.HasText("GitHub Work") || tt.HasText("Gitee Work") {
		t.Fatal("credential menu did not filter accounts by host")
	}
	if err := tt.Click("GitHub Work"); err != nil {
		t.Fatal(err)
	}
	if d.cloneCredentials.selected != "work" {
		t.Fatalf("wrong active credential: %q", d.cloneCredentials.selected)
	}
	if err := tt.Click("配置克隆凭据"); err != nil {
		t.Fatal(err)
	}
	if d.settingsTab != 6 || d.cloneCredentials.draft.Username != "work-user" || !tt.HasText("在浏览器授权") {
		t.Fatal("GitHub authorization settings missing")
	}
	saveLayoutPreview(t, tt, "clone-credentials-150.png")
	if !tt.HasText("GitHub Personal") || !tt.HasText("Gitee Work") {
		t.Fatal("settings credential list missing saved rows")
	}
	if err := tt.Click("删除凭据"); err != nil {
		t.Fatal(err)
	}
	if d.cloneCredentials.deleteID != "work" || !tt.HasText("确认删除") {
		t.Fatal("deletion is not scoped to selected profile")
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	if len(d.prefs.CloneProfiles) != 3 || d.cloneCredentials.deleteID != "" {
		t.Fatal("cancel changed saved profiles")
	}
	if err := tt.Click("新增凭据"); err != nil {
		t.Fatal(err)
	}
	if d.cloneCredentials.draft.ID != "" || d.cloneCredentials.draft.Username != "" {
		t.Fatal("adding did not clear editor")
	}

}

func TestProjectFormattingKeepsOriginal(t *testing.T) {
	d := New(nil, "", "")
	d.projectPreview.New = `{"x":[1,2]}`
	tt := native.NewTester(func(c *native.Context) { d.showCodePreview(c, "sample.json") }, 760, 460)
	if err := tt.Click("格式化预览"); err != nil {
		t.Fatal(err)
	}
	if !d.projectCode.ReadOnly || d.projectCode.Buffer.String() == d.projectPreview.New {
		t.Fatal("expected formatted read-only preview")
	}
	if err := tt.Click("显示原文"); err != nil {
		t.Fatal(err)
	}
	if d.projectCode.Buffer.String() != `{"x":[1,2]}` {
		t.Fatal("original JSON was changed")
	}
}

func TestGitHubRepositoryPickerSetsCloneSourceAndDestination(t *testing.T) {
	d := New(nil, "", "clone")
	d.cloneProvider = 1
	d.cloneAccount = "demo-user"
	d.cloneRepos = []string{"demo-user/visible", "other-org/hidden"}
	d.cloneRepoSearch = "visible"
	d.cloneParent = t.TempDir()
	d.prefs.CloneProfiles = []settings.CloneProfile{{ID: "demo", Label: "Demo", Kind: "github", Username: d.cloneAccount}}
	d.cloneCredentials.selected = "demo"
	tt := native.NewTester(d.view, 960, 680)
	if err := tt.Click("选择 GitHub 仓库"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("other-org/hidden") {
		t.Fatal("repository filter did not apply to the picker")
	}
	saveLayoutPreview(t, tt, "github-repository-picker.png")
	if err := tt.Click("demo-user/visible"); err != nil {
		t.Fatal(err)
	}
	if d.cloneURL != "demo-user/visible" || filepath.Base(d.cloneDestination) != "visible" {
		t.Fatalf("unexpected clone fields: %q, %q", d.cloneURL, d.cloneDestination)
	}
}

func TestHistoryPatchCanShowOneChangedFile(t *testing.T) {
	d := New(nil, "", "")
	d.setHistoryPatch("diff --git a/first.txt b/first.txt\n--- a/first.txt\n+++ b/first.txt\n@@ -1 +1 @@\n-old first\n+new first\n" +
		"diff --git a/second.txt b/second.txt\n--- a/second.txt\n+++ b/second.txt\n@@ -1 +1 @@\n-old second\n+new second\n")
	tt := native.NewTester(d.showHistoryPatch, 960, 460)
	if err := tt.Click("second.txt"); err != nil {
		t.Fatal(err)
	}
	lines := d.historyFilePatch.lines
	if len(lines) != 2 || lines[0].old != 1 || lines[0].text != "old second" || lines[1].next != 1 || lines[1].text != "new second" {
		t.Fatal("expected the patch for only the selected file")
	}
	saveLayoutPreview(t, tt, "history-file-patch.png")
}

func TestHunkDialogShowsCodeAndConfirmsDiscard(t *testing.T) {
	d := New(nil, "", "")
	d.hunksOpen, d.hunkPath, d.hunkPreviewRow = true, "service.go", -1
	d.hunks = []app.StageHunk{{Patch: "@@ -24 +24,2 @@ handler\n-old line\n+new line\n+next line\n"}}
	tt := native.NewTester(d.view, 960, 680)
	tt.SetScale(1.5)
	if !tt.HasText("旧行") || !tt.HasText("25") || tt.HasText("@@ -24 +24,2 @@ handler") {
		t.Fatal("expected old and new line numbers in the hunk preview")
	}
	saveLayoutPreview(t, tt, "structured-hunks-150.png")
	if err := tt.Click("丢弃此区块的改动"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("确认丢弃") || tt.HasText("暂存此区块") {
		t.Fatal("discard should require confirmation")
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("暂存此区块") {
		t.Fatal("cancel should preserve selection and restore actions")
	}
}

func TestFileHistoryShowsPatchAndBlameInColumns(t *testing.T) {
	d := New(nil, "", "")
	w := &d.tools
	w.fileOpen, w.filePath = true, "release.yml"
	w.fileCommits = []app.Commit{{Hash: "abcdef", Subject: "Update release", Author: "Alex", Date: "2026-11-18T09:00:00+08:00"}}
	w.filePatch.lines = []patchLine{{next: 8, kind: '+', text: "  release: true"}}
	w.blameLoaded = true
	w.blameRows = []app.BlameLine{{Number: 8, Author: "Alex", Date: "2026-11-18", Text: "  release: true"}}
	tt := native.NewTester(d.view, 960, 680)
	tt.SetScale(1.5)
	if !tt.HasText("Update release") || !tt.HasText("新行") || !tt.HasText("2026-11-18") {
		t.Fatal("missing file commit metadata or diff")
	}
	saveLayoutPreview(t, tt, "structured-file-history-150.png")
	if err := tt.Click(d.text("file.blame")); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("代码") || !tt.HasText("Alex") || tt.HasText("Update release") {
		t.Fatal("expected structured line annotations")
	}
}
