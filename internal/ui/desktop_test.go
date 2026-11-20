package ui

import (
	"bytes"
	"fmt"
	native "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"image/png"
	"mygittu/internal/app"
	git "mygittu/internal/git"
	"mygittu/internal/repository"
	"mygittu/internal/settings"
	"mygittu/internal/status"
	"os"
	"path/filepath"
	"testing"
)

func TestNativePages(t *testing.T) {
	for _, language := range []string{"zh-CN", "en"} {
		t.Run(language, func(t *testing.T) {
			for page := 0; page <= referencesPage; page++ {
				t.Run(fmt.Sprint(page), func(t *testing.T) {
					d := New(nil, "", "")
					d.applyPrefs(settings.Values{Language: language, Theme: "light"})
					d.selected = "demo"
					d.repos = []repository.Info{{ID: "demo", Root: "D:\\Code\\MyGittu"}}
					d.page = page
					if page == projectFilesPage {
						d.projectFiles = []string{"README.md", "internal/git/status.go"}
						d.rebuildProjectTree()
						d.projectFileRow = 0
						d.projectPreview.New = "# MyGittu"
					}
					if page == referencesPage {
						d.references = app.References{Stashes: []string{"hash\tstash@{0} Working changes"}, Tags: []string{"v1.0.0"}}
					}
					if page == 4 {
						d.page = 0
						d.settingsOpen = true
					}
					d.busy = page != projectFilesPage
					d.snapshot = status.Snapshot{Branch: "main"}
					d.changes = []git.FileStatus{{Path: "internal/git/status.go", Index: git.Modified, Worktree: git.Clean}, {Path: "README.md", Index: git.Clean, Worktree: git.Modified}, {Path: "中文文件.txt", Index: git.Clean, Worktree: git.Untracked}}
					d.stagedCount = 1
					d.logs = []app.Commit{{Hash: "0123456789012345678901234567890123456789", Subject: "Implement repository status", Author: "Developer", Date: "2026-11-17"}}
					d.detail = "* main\n  feature/status"
					d.row = 1
					d.comparison = &editor.DiffState{Old: editor.NewBuffer("# MyGittu\n\nA Git desktop client.\n"), New: editor.NewBuffer("# MyGittu\n\nA multilingual Git desktop client.\n\nSupports Windows and macOS.\n"), Layout: editor.DiffSplit}
					d.identity = app.Identity{Name: "Alex Chen", Email: "alex@example.invalid", Scope: "local", EffectiveName: "Alex Chen", EffectiveEmail: "alex@example.invalid"}
					d.commitIdentity = d.identity
					tt := native.NewTester(d.view, 1160, 740)
					if !tt.HasText("MyGittu") {
						t.Fatal("missing application title")
					}
					if directory := os.Getenv("MYGITTU_SNAPSHOT_DIR"); directory != "" {
						if e := os.MkdirAll(directory, 0700); e != nil {
							t.Fatal(e)
						}
						file, e := os.Create(filepath.Join(directory, fmt.Sprintf("%s-page-%d.png", language, page)))
						if e != nil {
							t.Fatal(e)
						}
						e = png.Encode(file, tt.Image())
						file.Close()
						if e != nil {
							t.Fatal(e)
						}
					}
				})
			}
		})
	}
}

func TestCommitAndCloneProgress(t *testing.T) {
	for _, operation := range []struct {
		action, key, label string
	}{
		{"commit", "commit.running", "正在提交…"},
		{"clone", "clone.running", "正在克隆…"},
	} {
		t.Run(operation.action, func(t *testing.T) {
			d := New(nil, "", operation.action)
			if operation.action == "commit" {
				d.selected = "demo"
			}
			if operation.action == "clone" {
				d.cloneURL = "https://github.com/example/demo.git"
				d.cloneDestination = filepath.Join(t.TempDir(), "demo")
			}
			d.busy, d.operation = true, operation.key
			tester := native.NewTester(d.view, 1160, 740)
			if operation.action == "commit" && (tester.HasText(operation.label) || !tester.HasText(d.text("commit.submit"))) {
				t.Fatal("commit progress should leave the workspace and its submit button visible")
			}
			if operation.action == "clone" && !tester.HasText(operation.label) {
				t.Fatal("operation progress is missing")
			}
			if operation.action == "clone" {
				label, _ := tester.Find(operation.label)
				before := bytes.Clone(tester.Image().Pix)
				d.operationFrame = 1
				tester.Frame()
				afterLabel, _ := tester.Find(operation.label)
				if bytes.Equal(before, tester.Image().Pix) || label != afterLabel {
					t.Fatal("clone animation must move the artwork and keep the status in place")
				}
				if tester.HasText(d.text("clone.title")) || !tester.HasText(d.cloneDestination) {
					t.Fatal("clone progress should show the destination in place of the form")
				}
				saveLayoutPreview(t, tester, "clone-running.png")
			}
			d.busy, d.operation = false, ""
			d.err = "operation failed"
			tester.Frame()
			if tester.HasText(operation.label) {
				t.Fatal("operation progress remains after failure")
			}
			if operation.action == "clone" &&
				(d.cloneURL != "https://github.com/example/demo.git" || !tester.HasText("配置克隆凭据") || !tester.HasText("操作失败: operation failed")) {
				t.Fatal("clone failure should restore the populated form and show the error")
			}
		})
	}
}

func TestRecentRepositorySearch(t *testing.T) {
	d := New(nil, "", "")
	d.repositoriesOpen = true
	d.repos = []repository.Info{
		{ID: "one", Root: "work/MyGittu", LastOpened: 2},
		{ID: "two", Root: "archive/MyGittu", LastOpened: 1},
		{ID: "discovered", Root: "external/MyGittu"},
	}
	tt := native.NewTester(d.view, 1160, 740)
	if !tt.HasText("work/MyGittu") || !tt.HasText("archive/MyGittu") || tt.HasText("external/MyGittu") {
		t.Fatal("recent projects could not be distinguished by path")
	}
	d.repositorySearch = "ARCHIVE"
	tt.Frame()
	if tt.HasText("work/MyGittu") || !tt.HasText("archive/MyGittu") {
		t.Fatal("search did not match the full repository path")
	}
}
func TestLanguageSwitchAndEmptySettings(t *testing.T) {
	d := New(nil, "", "")
	d.settingsOpen = true
	d.busy = true
	tt := native.NewTester(d.view, 1160, 740)
	if !tt.HasText("语言") {
		t.Fatal("settings unavailable without a repository")
	}
	d.applyPrefs(settings.Values{Language: "en", Theme: "light"})
	tt.Frame()
	if !tt.HasText("Language") || !tt.HasText("Open repository…") || tt.HasText("打开仓库") {
		t.Fatal("language switch did not update the interface")
	}
	d.applyPrefs(settings.Values{Language: "zh-CN", Theme: "light"})
	tt.Frame()
	if !tt.HasText("打开仓库…") {
		t.Fatal("cannot switch back")
	}
}

func TestSettingsOverlayPreservesWorkspace(t *testing.T) {
	d := New(nil, "", "")
	d.selected = "demo"
	d.page = 1
	d.row = 2
	d.message = "unfinished commit"
	d.query = "search"
	d.busy = true
	d.openSettings()
	tt := native.NewTester(d.view, 1160, 740)
	if !d.settingsOpen || !tt.HasText("语言") {
		t.Fatal("settings dialog did not open")
	}
	tt.Key(0, native.KeyEscape)
	tt.Frame()
	if d.settingsOpen {
		t.Fatal("Escape did not close settings")
	}
	if d.selected != "demo" || d.page != 1 || d.row != 2 || d.message != "unfinished commit" || d.query != "search" {
		t.Fatal("settings changed the workspace")
	}
}

func TestHostedCloneURL(t *testing.T) {
	for _, tc := range []struct {
		host, input, want string
		ssh               bool
	}{
		{"github.com", "octocat/Hello-World", "https://github.com/octocat/Hello-World.git", false},
		{"gitee.com", "https://gitee.com/team/project.git", "git@gitee.com:team/project.git", true},
		{"gitlab.com", "group/subgroup/project", "https://gitlab.com/group/subgroup/project.git", false},
		{"bitbucket.org", "git@bitbucket.org:team/project.git", "git@bitbucket.org:team/project.git", true},
		{"gitee.com", "https://github.com/team/project", "", false},
		{"github.com", "team/../project", "", false},
	} {
		if got := providerCloneURL(tc.host, tc.input, tc.ssh); got != tc.want {
			t.Errorf("%q -> %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestBusyWorkspaceHasNoGenericText(t *testing.T) {
	d := New(nil, "", "")
	d.busy = true
	tester := native.NewTester(d.view, 960, 680)
	if tester.HasText(d.text("busy")) {
		t.Fatal("generic busy text is still visible")
	}
}
