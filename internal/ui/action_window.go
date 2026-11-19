package ui

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	native "github.com/egoist/mygo/ui"
	"mygittu/internal/ipc"
	"mygittu/internal/repository"
	"mygittu/internal/settings"
)

func (d *Desktop) actionTitle() string {
	switch d.action {
	case "commit":
		return d.text("nav.commit")
	case "log":
		return d.text("nav.history")
	case "diff":
		return d.text("nav.diff")
	case "clone":
		return d.text("clone.title")
	}
	return "MyGittu"
}

func (d *Desktop) windowOptions() mygo.WindowOptions {
	options := mygo.WindowOptions{
		Title: d.actionTitle(), Width: 1280, Height: 820, MinWidth: 960, MinHeight: 680,
		Content: native.View(d.view),
	}
	switch d.action {
	case "clone":
		options.Width, options.Height = 720, 520
		options.MinWidth, options.MinHeight = 600, 480
	case "commit", "log", "diff":
		options.Width, options.Height = 960, 680
		options.MinWidth, options.MinHeight = 800, 560
	}
	return options
}

func (d *Desktop) loadAction() {
	path, action := d.path, d.action
	d.task(func(ctx context.Context) (func(), error) {
		var prefs settings.Values
		if err := d.Client.Call(ctx, "settings.get", ipc.Params{}, &prefs); err != nil {
			return nil, err
		}
		var repo repository.Info
		var registerErr error
		if action != "clone" {
			registerErr = d.Client.Call(ctx, "repo.register", ipc.Params{Path: path}, &repo)
		}
		return func() {
			d.applyPrefs(prefs)
			if registerErr != nil {
				d.err = registerErr.Error()
				return
			}
			if action != "clone" {
				if action == "diff" {
					d.initialDiffPath = repositoryRelativePath(repo, path)
				}
				d.selectRepository(repo.ID, repo.Root)
				d.refresh()
			}
		}, nil
	})
}

// Use the spelling recorded by Git's repository discovery for aliased roots.
func repositoryRelativePath(repo repository.Info, path string) string {
	for _, root := range append([]string{repo.Root}, repo.Aliases...) {
		rel, err := filepath.Rel(root, path)
		if err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return repository.Key(rel)
		}
	}
	return ""
}

func (d *Desktop) actionView(c *native.Context, t *native.Theme) {
	native.Column(c).Fill().Padding(panelSpacing).Gap(controlSpacing).Background(t.Surface).Children(func() {
		if d.result != "" {
			native.Column(c).Grow(1).FillWidth().Center().Gap(controlSpacing).Children(func() {
				native.Text(c, d.result).Bold()
				native.Text(c, d.path).FillWidth().SingleLine().Ellipsis("…").TextAlign(native.Center).Tooltip(d.path)
				if IconButton(c, d.text("close"), "close", true).Clicked() && d.win != nil {
					d.win.Close()
				}
			})
			return
		}
		if d.action != "clone" {
			native.Row(c).FillWidth().Gap(controlSpacing).AlignItems(native.Center).Children(func() {
				native.Text(c, d.path).Grow(1).MinWidth(0).SingleLine().Ellipsis("…").Tooltip(d.path).TextColor(t.TextMuted)
				if d.snapshot.Branch != "" {
					native.Text(c, d.snapshot.Branch).SingleLine()
				}
				if d.refreshButton(c).Disabled(d.busy || d.selected == "").Clicked() {
					d.refresh()
				}
			})
		}
		if d.err != "" && !d.settingsOpen {
			native.Scroll(c).FillWidth().MinHeight(34).MaxHeight(132).Children(func() {
				native.Text(c, d.text("error")+": "+d.err).FillWidth().TextColor(t.Danger)
			})
		}
		if d.operation == "commit.running" {
			native.Column(c).Grow(1).FillWidth().Center().Gap(controlSpacing).Children(func() {
				OperationIcon(c, 80, d.operationFrame, "commit")
				native.Text(c, d.text(d.operation)).Bold()
			})
			return
		}
		if d.action == "clone" {
			if d.operation == cloneOperation {
				d.cloneProgress(c)
			} else {
				native.Scroll(c).Grow(1).FillWidth().Children(func() { d.cloneView(c) })
			}
			return
		}
		if d.selected == "" {
			if d.busy {
				native.Text(c, d.text("busy")).TextColor(t.TextMuted)
			} else if d.err != "" && IconButton(c, d.text("refresh"), "refresh", false).Clicked() {
				d.loadAction()
			}
			return
		}
		switch d.action {
		case "commit", "diff":
			d.workspace(c, t)
		case "log":
			d.showHistory(c, t)
		}
	})
}
