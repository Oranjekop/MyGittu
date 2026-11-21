package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	native "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"mygittu/internal/app"
	"mygittu/internal/buildinfo"
	"mygittu/internal/commitai"
	git "mygittu/internal/git"
	"mygittu/internal/ipc"
	"mygittu/internal/repository"
	"mygittu/internal/settings"
	"mygittu/internal/status"
)

func TestProjectTabsKeepUnsavedText(t *testing.T) {
	d := New(nil, "", "")
	d.selected, d.page = "demo", projectFilesPage
	d.projectFiles = []string{".gitattributes", "docs/README.md"}
	d.projectExpanded = map[string]bool{"docs": true}
	d.rebuildProjectTree()
	text := &projectTab{path: "one.txt", preview: app.FilePreview{New: "original", Version: "8:1"}, code: codePreview("one.txt", "original"), loaded: true}
	other := &projectTab{path: "two.txt", preview: app.FilePreview{New: "other", Version: "5:1"}, code: codePreview("two.txt", "other"), loaded: true}
	d.projectTabs = map[string][]*projectTab{"demo": {text, other}}
	d.activateProjectTab(text)
	tt := native.NewTester(d.view, 960, 680)
	tt.SetScale(1.5)
	if d.projectCode.ReadOnly {
		t.Fatal("regular text did not become editable")
	}
	d.projectCode.Buffer.Set("unsaved text")
	tt.Frame()
	if !tt.HasText("one.txt*") {
		t.Fatal("modified tab lacks the asterisk")
	}
	tab, tabOK := tt.Find("one.txt*")
	filename, pathOK := tt.Find("one.txt")
	save, saveOK := tt.Find("保存")
	history, historyOK := tt.Find(d.text("file.history"))
	if !tabOK || !pathOK || !saveOK || !historyOK ||
		filename.Y < tab.Y+tab.H+controlSpacing ||
		save.Y <= filename.Y || history.Y != save.Y {
		t.Fatalf("tab %v (%t), path %v (%t), save %v (%t), history %v (%t)", tab, tabOK, filename, pathOK, save, saveOK, history, historyOK)
	}
	saveLayoutPreview(t, tt, "project-files-960-150.png")
	if err := tt.Click("docs"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if d.projectExpanded["docs"] || tt.HasText("README.md") {
		t.Fatal("clicking a folder in the file list should collapse its children")
	}
	if err := tt.Click("two.txt"); err != nil {
		t.Fatal(err)
	}
	if d.projectShownPath != "two.txt" || !d.hasUnsavedProjectFiles() {
		t.Fatal("switching tabs lost the pending edit")
	}
	if err := tt.Click("关闭 one.txt"); err != nil {
		t.Fatal(err)
	}
	if !d.repositoryTools.confirmOpen || d.findProjectTab("one.txt") == nil {
		t.Fatal("closing an edited tab must ask before discarding")
	}
}

func TestHistoryScrollRequestsMoreWithoutPageButtons(t *testing.T) {
	d := New(&ipc.Client{Dial: func(context.Context) (net.Conn, error) {
		return nil, context.Canceled
	}}, "", "")
	d.selected, d.page, d.historyMore = "demo", 1, true
	for i := range historyBatchSize {
		d.logs = append(d.logs, app.Commit{Hash: fmt.Sprintf("%04d", i), Subject: "commit"})
	}
	tt := native.NewTester(d.view, 960, 680)
	if d.historyLoading || tt.HasText(d.text("previous")) || tt.HasText(d.text("next")) {
		t.Fatal("history showed pagination or loaded without scrolling")
	}
	tt.Scroll(500, 300, 0, 4000)
	if !d.historyLoading {
		t.Fatal("scrolling near the bottom did not request more commits")
	}
}

func TestLargeProjectTextHasOneScrollbar(t *testing.T) {
	firstText := strings.Repeat("first section\n", 8000)
	lastText := strings.Repeat("second section\n", 12000) + "最后一行"
	client := &ipc.Client{Dial: func(context.Context) (net.Conn, error) {
		reader, writer := net.Pipe()
		go func() {
			defer writer.Close()
			var request ipc.Request
			if err := ipc.ReadFrame(writer, &request); err != nil {
				t.Error(err)
				return
			}
			var params ipc.Params
			if err := json.Unmarshal(request.Params, &params); err != nil {
				t.Error(err)
				return
			}
			if params.PreviewOffset != int64(len(firstText)) || params.PreviewVersion != "same-file" {
				t.Errorf("unexpected document cursor: %+v", params)
			}
			payload, err := json.Marshal(app.FilePreview{New: lastText, Version: "same-file"})
			if err != nil {
				t.Error(err)
				return
			}
			if err := ipc.WriteFrame(writer, ipc.Response{Version: ipc.Version, ID: request.ID, OK: true, Result: payload}); err != nil {
				t.Error(err)
			}
		}()
		return reader, nil
	}}
	code, err := readProjectText(context.Background(), client, ipc.Params{Path: "large.txt"}, app.FilePreview{
		New: firstText, NextOffset: int64(len(firstText)), Version: "same-file",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := code.Buffer.Line(code.Buffer.LineCount() - 1); got != "最后一行" || code.Buffer.LineCount() != 20001 {
		t.Fatalf("the editor lost content: %q, %d lines", got, code.Buffer.LineCount())
	}
	d := New(nil, "D:/Projects/MyGittu", "")
	d.selected, d.page, d.projectFileRow = "demo", projectFilesPage, 0
	d.projectFiles = []string{"large.txt"}
	d.rebuildProjectTree()
	d.projectFileRow = 0
	d.projectCode, d.projectCodePath, d.projectLargeText = code, "large.txt", true
	tt := native.NewTester(d.view, 960, 680)
	for notch := 1; notch <= 12; notch++ {
		tt.Wheel(650, 250, 0, 180)
		tt.Frame()
		if y := code.ScrollY(); y != float64(notch*180) {
			t.Fatalf("wheel event %d moved to %f, want %d immediately", notch, y, notch*180)
		}
	}
	tt.Scroll(650, 250, 0, -40)
	tt.Frame()
	if y := code.ScrollY(); y != 2120 {
		t.Fatalf("precise scroll moved to %f, want 2120", y)
	}
	code.SetScrollY(0)
	tt.Frame()
	tt.Press(935, 126)
	tt.Move(935, 658)
	tt.Release(935, 658)
	if code.ScrollY() < 100000 {
		t.Fatalf("scrollbar did not travel beyond the first data chunk: %f", code.ScrollY())
	}
}

func TestWorkspaceCloneDialogAt150Percent(t *testing.T) {
	d := New(nil, "", "")
	d.selected, d.page = "demo", 1
	d.cloneURL = "https://example.invalid/demo.git"
	d.cloneDestination = t.TempDir()
	tt := native.NewTester(d.view, 960, 680)
	tt.SetScale(1.5)
	if err := tt.Click(d.text("clone.run")); err != nil {
		t.Fatal(err)
	}
	if !d.cloneOpen || d.page != 1 || !tt.HasText(d.text("clone.title")) || !tt.HasText("配置克隆凭据") {
		t.Fatal("clone form did not open over the current page")
	}
	closeButton, closeOK := tt.Find(d.text("close"))
	runButton, runOK := tt.Find("配置克隆凭据")
	if !closeOK || !runOK || absLayout(closeButton.X+closeButton.W-runButton.X-runButton.W) > 0.5 {
		t.Fatalf("clone form and dialog header end at different edges: %v, %v", closeButton, runButton)
	}
	saveLayoutPreview(t, tt, "workspace-clone-dialog.png")
	if err := tt.Click("配置克隆凭据"); err != nil {
		t.Fatal(err)
	}
	if !d.settingsOpen || d.settingsTab != settingsCloneCredentials {
		t.Fatal("clone credential settings did not open")
	}
	tt.Key(0, native.KeyEscape)
	if d.settingsOpen || !d.cloneOpen || !tt.HasText("配置克隆凭据") || d.cloneURL != "https://example.invalid/demo.git" {
		t.Fatal("closing settings did not restore the clone dialog")
	}
	d.operation, d.busy = cloneOperation, true
	tt.Frame()
	tt.Key(0, native.KeyEscape)
	if !d.cloneOpen || !tt.HasText(d.text(cloneOperation)) {
		t.Fatal("closing the dialog interrupted the progress presentation")
	}
	d.operation, d.busy = "", false
	tt.Frame()
	tt.Key(0, native.KeyEscape)
	if d.cloneOpen || d.page != 1 || d.selected != "demo" {
		t.Fatal("closing clone changed the active workspace")
	}
}

func TestTableSeparatorKeepsHighlightedRowSpaced(t *testing.T) {
	selected := 0
	list := native.ListState{Selected: &selected}
	var border, accent color.RGBA
	tt := native.NewTester(func(c *native.Context) {
		theme := ApplyTheme(c, 12.5)
		border = color.RGBA{R: theme.Border.R, G: theme.Border.G, B: theme.Border.B, A: theme.Border.A}
		accent = color.RGBA{R: theme.Accent.R, G: theme.Accent.G, B: theme.Accent.B, A: theme.Accent.A}
		native.Column(c).Fill().Padding(panelSpacing).Background(theme.Background).Children(func() {
			roundedTable(c, &list, []native.TableColumn{{Title: "Name"}}, 1, func(_, _ int) {
				native.Text(c, "Selected")
			}).Grow(1)
		})
	}, 400, 240)
	for _, scale := range []float32{1, 1.5, 2} {
		tt.SetScale(scale)
		img := tt.Image()
		x := int(200 * scale)
		borderEnd, rowStart := -1, -1
		for y := range img.Bounds().Dy() {
			pixel := img.RGBAAt(x, y)
			if pixel == border {
				borderEnd = y
			}
			if pixel == accent {
				rowStart = y
				break
			}
		}
		if borderEnd < 0 || rowStart-borderEnd-1 != int(controlSpacing*scale) {
			t.Fatalf("scale %.1f: divider ends at %d, selected row starts at %d", scale, borderEnd, rowStart)
		}
	}
}

func TestTagSortingKeepsTheSelectedTag(t *testing.T) {
	d := New(nil, "", "")
	d.selected, d.page = "demo", referencesPage
	d.references = app.References{
		Tags: []string{"v0.0.19", "v0.0.9", "no-date"},
		TagTimes: map[string]app.TagTime{
			"v0.0.19": {Unix: 1795052051, Annotated: true},
			"v0.0.9":  {Unix: 1795050000},
		},
	}
	tt := native.NewTester(d.view, 960, 680)
	tt.SetScale(1.5)
	if tags := d.sortedTagNames(); tags[0] != "v0.0.19" || !d.tagList.RoundRows || !tt.HasText(d.text("tag.time")) {
		t.Fatal("tag version order, rounded rows or time column missing")
	}
	if err := tt.Click("v0.0.19"); err != nil {
		t.Fatal(err)
	}
	saveLayoutPreview(t, tt, "tags-first-selected.png")
	if err := tt.Click(d.text("tag.time")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tags := d.sortedTagNames(); d.tagSort.Column != "time" || tags[0] != "v0.0.9" || tags[2] != "no-date" || d.tagRow != 1 {
		t.Fatal("time sorting moved the selection to a different tag")
	}
	saveLayoutPreview(t, tt, "tags-with-time.png")
}

func TestProjectPreviewStaysVisibleDuringSelection(t *testing.T) {
	d := New(nil, "D:/Projects/MyGittu", "")
	d.selected, d.page = "demo", projectFilesPage
	d.projectFiles = []string{"first.txt", "second.txt"}
	d.rebuildProjectTree()
	d.projectFileRow, d.projectShownPath = 0, "first.txt"
	d.projectPreview.New = "The first file stays visible."
	tt := native.NewTester(d.view, 960, 680)
	editor := d.projectCode
	before, ok := tt.Find("自动换行")
	if !ok || editor == nil {
		t.Fatal("missing initial editor")
	}
	d.projectFileRow = 1
	d.projectLoad = &projectPreviewRequest{cancel: func() {}}
	tt.Frame()
	if after, ok := tt.Find("自动换行"); !ok || after != before || d.projectCode != editor || d.projectCodePath != "first.txt" {
		t.Fatal("loading a different file removed, relabeled or moved the current editor")
	}
}

func TestProjectFileDividerRemembersWidth(t *testing.T) {
	d := New(nil, "", "")
	d.selected, d.page = "demo", projectFilesPage
	d.projectFiles = []string{"README.md"}
	d.rebuildProjectTree()
	tt := native.NewTester(d.view, 960, 680)
	tt.SetScale(1.5)
	divider := float32(2*panelSpacing + sidebarWidth + defaultProjectPaneWidth)
	tt.Press(divider, 320)
	tt.Move(divider+60, 320)
	tt.Release(divider+60, 320)
	if !d.projectPaneChanged || d.projectPane <= defaultProjectPaneWidth || d.filePane != defaultFilePaneWidth {
		t.Fatalf("file divider drag did not change only the project width: %v, %v", d.projectPane, d.filePane)
	}
	path := filepath.Join(t.TempDir(), "project-file-pane.json")
	saveFilePane(path, d.projectPane, d.projectPaneChanged)
	reopened := New(nil, "", "")
	restoreFilePane(path, &reopened.projectPane)
	if reopened.projectPane != d.projectPane {
		t.Fatalf("restored width = %v, want %v", reopened.projectPane, d.projectPane)
	}
	saveLayoutPreview(t, tt, "project-file-divider.png")
}

func TestProjectContentResultOpensFileAtLine(t *testing.T) {
	d := New(nil, "", "")
	d.selected, d.page = "demo", projectFilesPage
	tab := &projectTab{path: "test.txt", code: codePreview("test.txt", "zero\none\nmatch\n"), loaded: true}
	d.projectTabs = map[string][]*projectTab{d.selected: {tab}}
	d.fileSearch = projectContentSearch{open: true, found: true, row: -1, result: app.FileSearch{
		Matches: []app.FileMatch{{Path: tab.path, Line: 3, Text: "match"}},
	}}
	tt := native.NewTester(d.view, 960, 680)
	tt.SetScale(1.5)
	saveLayoutPreview(t, tt, "file-content-search.png")
	if err := tt.Click("test.txt:3"); err != nil {
		t.Fatal(err)
	}
	if d.fileSearch.open || d.projectShownPath != tab.path || d.projectCode.CursorPos().Line != 2 {
		t.Fatalf("search result did not reveal line 3: %q, %+v", d.projectShownPath, d.projectCode.CursorPos())
	}
}

func TestStashPreviewSelectsIndividualPatch(t *testing.T) {
	d := New(nil, "", "")
	d.selected, d.page = "demo", referencesPage
	d.stashPreview = stashPreview{open: true, label: "stash@{0} preview", row: -1, files: []editor.FilePatch{
		{OldName: "one.txt", NewName: "one.txt", Binary: true},
		{OldName: "two.txt", NewName: "two.txt"},
	}}
	tt := native.NewTester(d.view, 960, 680)
	tt.SetScale(1.5)
	if err := tt.Click("two.txt"); err != nil {
		t.Fatal(err)
	}
	if d.stashPreview.row != 1 || len(d.stashPreview.patch.lines) != 1 || !tt.HasText("文件属性或路径变更") {
		t.Fatal("stash did not show the selected file's patch")
	}
	saveLayoutPreview(t, tt, "stash-file-preview.png")
}

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
	dividerX := 2*panelSpacing + sidebarWidth + d.filePane
	tt.Press(float32(dividerX), 318)
	tt.Move(float32(dividerX-25), 318)
	tt.Release(float32(dividerX-25), 318)
	tt.Move(20, 20)
	if d.filePane >= 380 {
		t.Fatal("dragging the file divider did not resize the pane")
	}
	layoutPath := filepath.Join(t.TempDir(), "workspace-file-pane.json")
	saveFilePane(layoutPath, d.filePane, d.filePaneChanged)
	reopened := New(nil, "", "")
	restoreFilePane(layoutPath, &reopened.filePane)
	if reopened.filePane != d.filePane {
		t.Fatalf("reopened divider width = %v, want %v", reopened.filePane, d.filePane)
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
	if _, ok := tt.Find("展开或收起侧栏"); ok {
		t.Fatal("sidebar collapse control is still visible")
	}
	if bounds, ok := tt.Find("文件"); !ok || bounds.X != panelSpacing || bounds.W != sidebarWidth {
		t.Fatalf("sidebar button should be directly in the outer layout: %v", bounds)
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
	compare, compareOK := tt.Find(d.text("compare.useSelected"))
	switchButton, switchOK := tt.Find(d.text("branch.switch"))
	merge, mergeOK := tt.Find(d.text("branch.merge"))
	remove, removeOK := tt.Find(d.text("branch.delete"))
	if !compareOK || !switchOK || !mergeOK || !removeOK ||
		compare.X != merge.X || switchButton.X != remove.X ||
		compare.Y != switchButton.Y || merge.Y != remove.Y || compare.W != remove.W {
		t.Fatalf("branch buttons do not form an aligned grid: %v %v %v %v", compare, switchButton, merge, remove)
	}
	d.tools.compared = true
	d.tools.comparison = app.BranchComparisonSummary{BranchComparisonPage: app.BranchComparisonPage{Commits: []app.BranchComparisonCommit{
		{Side: "<", Commit: app.Commit{Subject: "修订主分支说明"}},
		{Side: ">", Commit: app.Commit{Subject: "支持工作区调整"}},
	}}}
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
	tt.SetScale(1.5)
	remove, removeOK := tt.Find(d.text("repo.forget"))
	open, openOK := tt.Find(d.text("repo.open") + "…")
	if !removeOK || !openOK || absLayout(remove.X+remove.W-open.X-open.W) > 0.5 {
		t.Fatalf("recent repository remove button is inset from the dialog controls: %v, %v", remove, open)
	}
	saveLayoutPreview(t, tt, "recent-repositories-150.png")
	for i := range 12 {
		d.repos = append(d.repos, repository.Info{ID: fmt.Sprint(i), Root: fmt.Sprintf("work/overflow-%d", i), LastOpened: 1})
	}
	tt.Frame()
	tt.Scroll(500, 300, 0, 2000)
	tt.Frame()
	if !tt.HasText("overflow-11") {
		t.Fatal("the overflowing recent repository list cannot scroll to the last row")
	}
}

func absLayout(value float32) float32 { return max(value, -value) }

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
	credentials, credentialsOK := tt.Find("克隆凭据")
	update, updateOK := tt.Find(d.text("update.title"))
	if !credentialsOK || !updateOK || update.Y <= credentials.Y {
		t.Fatal("software update should be below clone credentials")
	}
	saveLayoutPreview(t, tt, "settings-appearance-150.png")
	if err := tt.Click("AI 配置"); err != nil {
		t.Fatal(err)
	}
	if d.settingsTab != settingsAI || !tt.HasText("供应商") {
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
	if d.settingsTab != settingsAI || !tt.HasText("提交信息提示词") || d.aiPromptDraft != commitai.DefaultPrompt() {
		t.Fatal("AI settings did not show the default prompt")
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
	if !d.settingsOpen || d.settingsTab != settingsAI {
		t.Fatal("unconfigured AI should open provider settings")
	}
}

func TestIntegrationSettingsAt150Percent(t *testing.T) {
	d := New(nil, "", "")
	d.openSettings()
	d.settingsTab = settingsIntegration
	d.integrationLoaded = true
	d.integration = integrationState{Menu: true, Modern: true, Overlays: true}
	tt := native.NewTester(d.view, 960, 680)
	tt.SetScale(1.5)
	for _, label := range []string{"系统集成", "登录时启动 MyGittu"} {
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
	if err := tt.Click("软件更新"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("当前版本  " + buildinfo.Version) {
		t.Fatal("installed version missing from software update settings")
	}
	if err := tt.Click("克隆凭据"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("暂无凭据") || !tt.HasText("新建凭据") {
		t.Fatal("empty credential list or editor missing")
	}
	saveLayoutPreview(t, tt, "clone-credentials-empty-150.png")
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
	if d.settingsTab != settingsCloneCredentials || d.cloneCredentials.draft.Username != "work-user" || !tt.HasText("在浏览器授权") {
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
	diff := &d.historyFileDiff
	if diff.Old == nil || diff.New == nil || strings.TrimSpace(diff.Old.String()) != "old second" || strings.TrimSpace(diff.New.String()) != "new second" {
		t.Fatal("expected the patch for only the selected file")
	}
	if diff.Layout != editor.DiffSplit {
		t.Fatal("expected side-by-side history diff in a wide window")
	}
	saveLayoutPreview(t, tt, "history-file-patch.png")
	if err := tt.Click("统一"); err != nil {
		t.Fatal(err)
	}
	if diff.Layout != editor.DiffUnified {
		t.Fatal("expected unified history diff after changing views")
	}
}

func TestCloseHistoryDetails(t *testing.T) {
	d := New(nil, "", "")
	d.selected, d.page, d.logRow = "demo", 1, 0
	d.logs = []app.Commit{{Hash: "example", Subject: "Example commit"}}
	d.historyShown = d.logs[0]
	summary := "Example commit\n\nDetails body"
	d.historySummary = &summary
	tt := native.NewTester(d.view, 960, 680)
	if !tt.HasText("Details body") {
		t.Fatal("expected commit details before closing")
	}
	divider, ok := tt.AccessByRole(native.RoleSplitter, "")
	if !ok {
		t.Fatal("expected a divider with commit details open")
	}
	x, y := divider.Bounds.X+divider.Bounds.W/2, divider.Bounds.Y+divider.Bounds.H/2
	tt.Press(x, y)
	tt.Move(x, y+70)
	tt.Release(x, y+70)
	if d.historySplit <= 50 {
		t.Fatal("dragging down should give more space to the commit list")
	}
	if err := tt.Click("关闭提交详情"); err != nil {
		t.Fatal(err)
	}
	if d.logRow != -1 || d.historySummary != nil || tt.HasText("Details body") || !tt.HasText("Example commit") {
		t.Fatal("closing should clear the selection and hide details while keeping the commit list")
	}
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

func TestCommitHistoryFormatsTheSameDateInListAndDetails(t *testing.T) {
	d := New(nil, "", "")
	d.selected, d.page, d.logRow = "demo", 1, 0
	const timestamp = "2026-11-18T16:00:21+08:00"
	d.logs = []app.Commit{{Hash: "abc", Author: "Developer", Subject: "Commit", Date: timestamp}}
	summary := "Commit"
	d.historySummary = &summary
	tt := native.NewTester(d.view, 960, 680)
	expected := time.Date(2026, 11, 18, 8, 0, 21, 0, time.UTC).Local().Format(time.DateTime)
	if !tt.HasText(expected) || !tt.HasText("Developer · "+expected) || tt.HasText(timestamp) {
		t.Fatal("history exposed an unformatted timestamp")
	}
}

func TestHistoryGraphShowsMergeAndBranchLanes(t *testing.T) {
	d := New(nil, "", "")
	d.selected, d.page = "demo", 1
	d.logs = []app.Commit{
		{Hash: "merge", Subject: "Merge feature", Parents: []string{"main", "feature"}, Refs: "HEAD -> main"},
		{Hash: "feature", Subject: "Feature change", Parents: []string{"base"}, Refs: "feature"},
		{Hash: "main", Subject: "Main change", Parents: []string{"base"}},
		{Hash: "base", Subject: "Base"},
	}
	d.historyGraph, d.historyGraphWidth = buildCommitGraph(d.logs, false)
	if merge, feature := d.historyGraph[0], d.historyGraph[1]; !merge.merge || len(merge.outgoing) != 2 || feature.lane == merge.lane || !feature.incoming {
		t.Fatal("graph lost a merge parent or its branch lane")
	}
	tt := native.NewTester(d.view, 960, 680)
	tt.SetScale(1.5)
	if !tt.HasText("分支图") || !tt.HasText("HEAD -> main") {
		t.Fatal("missing graph column or branch label")
	}
	if first, ok := tt.Find("merge"); ok {
		tt.Move(first.X+first.W/2, first.Y+first.H/2)
		tt.Frame()
	}
	saveLayoutPreview(t, tt, "history-graph.png")
}

func TestFileHistoryShowsPatchAndBlameInColumns(t *testing.T) {
	d := New(nil, "", "")
	w := &d.tools
	w.fileOpen, w.filePath = true, "release.yml"
	w.fileCommits = []app.Commit{{Hash: "abcdef", Subject: "Update release", Author: "Alex", Date: "2026-11-18T09:00:00+08:00"}}
	w.filePatch.lines = []patchLine{{next: 8, kind: '+', text: "  release: true"}}
	w.blameLoaded = true
	w.blameRows = []app.BlameLine{{Number: 8, Author: "Alex", Date: "2026-11-18T09:00:00+08:00", Text: "  release: true"}}
	expected := time.Date(2026, 11, 18, 1, 0, 0, 0, time.UTC).Local().Format(time.DateTime)
	tt := native.NewTester(d.view, 960, 680)
	tt.SetScale(1.5)
	if !tt.HasText("Update release") || !tt.HasText("新行") || !tt.HasText(expected) {
		t.Fatal("missing file commit metadata or diff")
	}
	saveLayoutPreview(t, tt, "structured-file-history-150.png")
	if err := tt.Click(d.text("file.blame")); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("代码") || !tt.HasText("Alex") || !tt.HasText(expected) || tt.HasText("Update release") {
		t.Fatal("expected structured line annotations")
	}
}

func TestShellActionWindowsAt150Percent(t *testing.T) {
	for _, tc := range []struct{ action, control string }{
		{"commit", "提交（1）"},
		{"log", "搜索"},
		{"diff", "全部暂存"},
		{"clone", "开始克隆"},
	} {
		t.Run(tc.action, func(t *testing.T) {
			d := New(nil, "D:/Projects/MyGittu", tc.action)
			if tc.action != "clone" {
				d.selected, d.snapshot.Branch = "demo", "main"
			}
			switch tc.action {
			case "commit", "diff":
				d.changes = []git.FileStatus{
					{Path: "README.md", Index: git.Modified, Worktree: git.Clean, State: git.Modified},
					{Path: "internal/ui/action_window.go", Index: git.Clean, Worktree: git.Modified, State: git.Modified},
					{Path: "packaging/windows/integration.ps1", Index: git.Clean, Worktree: git.Modified, State: git.Modified},
				}
				d.row, d.stagedCount, d.staged = 0, 1, true
				d.message = "让右键操作在独立窗口中完成"
				d.commitIdentity.EffectiveName = "Alex"
				d.applyComparison("README.md", app.Comparison{
					Old: "# MyGittu\n\n在资源管理器中打开仓库。\n\n## 安装\n\n运行安装包后，在设置中启用右键菜单。\n",
					New: "# MyGittu\n\n在资源管理器中打开仓库。\n提交、克隆、历史和差异各自在独立窗口中打开。\n\n## 安装\n\n运行安装包后，在设置中启用右键菜单。\n",
				})
			case "log":
				d.logs = []app.Commit{
					{Hash: "91f8e230ccba", Subject: "让右键操作在独立窗口中完成", Author: "Alex", Date: "2026-11-19"},
					{Hash: "c30bfce405d3", Subject: "修复拉取后的资源管理器徽章", Author: "Alex", Date: "2026-11-18"},
					{Hash: "e9a2f7ac11da", Subject: "简化工作区侧栏", Author: "Alex", Date: "2026-11-18"},
				}
				d.logRow = 0
				summary := "让右键操作在独立窗口中完成\n\n将提交、克隆、历史和差异接入单独的窗口。"
				d.historySummary = &summary
			case "clone":
				d.cloneParent = ""
				d.cloneURL = "https://github.com/example/demo.git"
				d.cloneDestination = "D:/Projects/demo"
			}
			options := d.windowOptions()
			tt := native.NewTester(d.view, int(options.MinWidth), int(options.MinHeight))
			tt.SetScale(1.5)
			if bounds, ok := tt.Find(tc.control); !ok || bounds.W <= 0 || bounds.H <= 0 ||
				bounds.X < 0 || bounds.Y < 0 || bounds.X+bounds.W > float32(options.MinWidth) || bounds.Y+bounds.H > float32(options.MinHeight) {
				t.Fatalf("action control %q is not visible: %v", tc.control, bounds)
			}
			if _, ok := tt.Find(d.text("nav.branches")); ok {
				t.Fatal("standalone action displayed the workspace navigation")
			}
			if tc.action == "clone" {
				for _, width := range []int{int(options.MinWidth), 1000} {
					tt.SetSize(width, int(options.Height))
					button, ok := tt.Find(tc.control)
					if !ok || absLayout(2*button.X+button.W-float32(width)) > 0.5 {
						t.Fatalf("standalone clone form is off-center at width %d: %v", width, button)
					}
				}
			}
			if os.Getenv("MYGITTU_LAYOUT_PREVIEW") != "" {
				tt.SetSize(int(options.Width), int(options.Height))
				saveLayoutPreview(t, tt, "context-menu-"+tc.action+".png")
			}
			if tc.action == "commit" {
				if err := tt.Click("提交选项"); err != nil {
					t.Fatal(err)
				}
				if !tt.HasText("提交并推送") {
					t.Fatal("missing Commit & Push action")
				}
			}
			if tc.action == "commit" || tc.action == "clone" {
				d.result = "操作完成"
				tt.Frame()
				if !tt.HasText(d.result) || !tt.HasText(d.text("close")) || tt.HasText(tc.control) {
					t.Fatal("successful action did not stay in its own result window")
				}
			}
		})
	}
}

func TestProjectFileTypeIcons(t *testing.T) {
	for name, want := range map[string]string{
		"main.go": "file-code", "Makefile": "file-code", "README.md": "changes",
		".gitignore": "file-data", "config.YAML": "file-data", "banner.PNG": "file-image",
		"demo.mp4": "file-media", "export.tar.gz": "file-archive", "unknown.bin": "file",
	} {
		if got := projectFileIcon(name); got != want || icons[got] == nil {
			t.Errorf("%s icon = %q, want %q", name, got, want)
		}
	}
	d := New(nil, "D:/Projects/MyGittu", "")
	d.selected, d.page = "demo", projectFilesPage
	d.projectFiles = []string{
		"Makefile", "README.md", "assets/banner.PNG", "assets/demo.mp4",
		"config/settings.yaml", "dist/export.zip", "src/main.go", "src/nested/child.go",
	}
	d.projectExpanded = map[string]bool{"assets": true, "src": true}
	d.rebuildProjectTree()
	wantPaths := []string{"assets", "assets/banner.PNG", "assets/demo.mp4", "config", "dist", "src", "src/nested", "src/main.go", "Makefile", "README.md"}
	if len(d.projectTree) != len(wantPaths) {
		t.Fatalf("unexpected visible file tree: %v", d.projectTree)
	}
	for i, want := range wantPaths {
		if d.projectTree[i].path != want {
			t.Fatalf("row %d = %q, want %q", i, d.projectTree[i].path, want)
		}
	}
	tt := native.NewTester(d.view, 960, 680)
	tt.SetScale(1.5)
	if !tt.HasText("banner.PNG") || !tt.HasText("main.go") || tt.HasText("export.zip") {
		t.Fatal("file rows were clipped or a collapsed folder exposed its contents")
	}
	saveLayoutPreview(t, tt, "project-file-type-icons.png")
	if err := tt.Click("dist"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("export.zip") {
		t.Fatal("folder row did not expand")
	}
}

func TestProjectImagePreviewFitsPane(t *testing.T) {
	d := New(nil, "D:/Projects/MyGittu", "")
	d.selected, d.page, d.projectFileRow = "demo", projectFilesPage, 0
	d.projectFiles = []string{"wide.png"}
	d.rebuildProjectTree()
	d.projectFileRow = 0
	source := image.NewRGBA(image.Rect(0, 0, 1024, 256))
	ink := color.RGBA{R: 241, G: 47, B: 31, A: 255}
	draw.Draw(source, source.Bounds(), &image.Uniform{C: ink}, image.Point{}, draw.Src)
	d.projectBitmap = native.NewBitmap(source)
	tt := native.NewTester(d.view, 960, 680)
	tt.SetScale(1.5)
	if _, ok := tt.Find("外部打开"); !ok {
		t.Fatal("external open action missing from image preview")
	}
	rendered := tt.Image()
	var painted image.Rectangle
	for y := rendered.Bounds().Min.Y; y < rendered.Bounds().Max.Y; y++ {
		for x := rendered.Bounds().Min.X; x < rendered.Bounds().Max.X; x++ {
			if rendered.RGBAAt(x, y) == ink {
				painted = painted.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	aspectError := painted.Dx() - 4*painted.Dy()
	if painted.Dx() < 400 || painted.Min.X < 590 || aspectError < -8 || aspectError > 8 {
		t.Fatalf("image did not fit the preview pane at its original aspect ratio: %v", painted)
	}
}
