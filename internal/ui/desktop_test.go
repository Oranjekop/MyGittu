package ui

import (
	"fmt"
	native "github.com/egoist/mygo/ui"
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
			for page := 0; page < 5; page++ {
				t.Run(fmt.Sprint(page), func(t *testing.T) {
					d := New(nil, "", "")
					d.applyPrefs(settings.Values{Language: language, Theme: "light"})
					d.selected = "demo"
					d.repos = []repository.Info{{ID: "demo", Root: "D:\\Code\\MyGittu"}}
					d.page = page
					d.busy = true
					d.snapshot = status.Snapshot{Branch: "main"}
					d.changes = []git.FileStatus{{Path: "internal/git/status.go", Index: git.Modified, Worktree: git.Clean}, {Path: "README.md", Index: git.Clean, Worktree: git.Modified}, {Path: "中文文件.txt", Index: git.Clean, Worktree: git.Untracked}}
					d.stagedCount = 1
					d.logs = []app.Commit{{Hash: "0123456789012345678901234567890123456789", Subject: "Implement repository status", Author: "Developer", Date: "2026-11-17"}}
					d.detail = "* main\n  feature/status"
					d.identity = app.Identity{Name: "Alex Chen", Email: "alex@example.invalid", Scope: "local", EffectiveName: "Alex Chen", EffectiveEmail: "alex@example.invalid"}
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
func TestLanguageSwitchAndEmptySettings(t *testing.T) {
	d := New(nil, "", "")
	d.page = 4
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
