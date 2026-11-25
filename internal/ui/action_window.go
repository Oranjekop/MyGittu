package ui

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	native "github.com/egoist/mygo/ui"
	git "mygittu/internal/git"
	"mygittu/internal/ipc"
	"mygittu/internal/repository"
	"mygittu/internal/settings"
	"mygittu/internal/status"
)

func (d *Desktop) actionTitle() string {
	if strings.HasPrefix(d.action, "file-") {
		return d.text(d.action)
	}
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
		StateKey: "workspace",
		Content:  native.View(d.view),
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		options.TitleBarStyle = mygo.TitleBarHidden
	}
	if runtime.GOOS == "darwin" && d.action == "" {
		options.TrafficLightPosition = &mygo.Point{X: panelSpacing, Y: panelSpacing + 9}
	}
	switch d.action {
	case "clone":
		options.StateKey = d.action
		options.Width, options.Height = 720, 520
		options.MinWidth, options.MinHeight = 600, 480
	case "commit", "log", "diff":
		options.StateKey = d.action
		options.Width, options.Height = 960, 680
		options.MinWidth, options.MinHeight = 800, 560
	}
	if strings.HasPrefix(d.action, "file-") {
		options.StateKey = d.action
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
		var snapshot status.Snapshot
		var registerErr error
		if action != "clone" {
			registerErr = d.Client.Call(ctx, "repo.register", ipc.Params{Path: path}, &repo)
		}
		if strings.HasPrefix(action, "file-") && action != "file-log" && action != "file-blame" && registerErr == nil {
			registerErr = d.Client.Call(ctx, "status.refresh", ipc.Params{RepoID: repo.ID}, &snapshot)
		} else if action == "log" && registerErr == nil {
			if err := d.Client.Call(ctx, "status.get", ipc.Params{RepoID: repo.ID}, &snapshot); err != nil {
				_ = d.Client.Call(ctx, "status.refresh", ipc.Params{RepoID: repo.ID}, &snapshot)
			}
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
				if strings.HasPrefix(action, "file-") {
					d.openFileAction(path, repo, snapshot)
					return
				}
				if snapshot.CherryPicking {
					d.page = 0
					d.applyWorkspaceStatus(snapshot)
				}
				d.refresh()
			}
		}, nil
	})
}

func (d *Desktop) openFileAction(path string, repo repository.Info, snapshot status.Snapshot) {
	target := repositoryRelativePath(repo, path)
	if target == "" {
		d.err = d.text("file.actionUnavailable")
		return
	}
	switch d.action {
	case "file-log":
		d.openFileHistory(target)
		return
	case "file-blame":
		d.tools.filePath, d.tools.fileMode, d.tools.fileOpen = target, 1, true
		return
	}
	d.applyWorkspaceStatus(snapshot)
	for i, file := range d.changes {
		if repository.Key(file.Path) != target {
			continue
		}
		d.row, d.staged = i, file.Worktree == git.Clean
		switch d.action {
		case "file-diff":
			d.diff(file.Path)
			return
		case "file-conflict":
			if file.State == git.Conflicted {
				d.loadConflict(file.Path)
				return
			}
		case "file-stage", "file-unstage":
			if paths := stagingPaths(nil, file, d.action == "file-stage"); len(paths) > 0 {
				method := "git.stage"
				if d.action == "file-unstage" {
					method = "git.unstage"
				}
				d.completeFileAction(ipc.Params{Paths: paths}, method, file.Path)
				return
			}
		case "file-restore":
			if file.Worktree != git.Clean && file.Worktree != git.Untracked && file.State != git.Conflicted && file.Index != git.Deleted {
				d.confirmFileRestore(file.Path, "tracked", d.text("file.restoreConfirm"))
				return
			}
		}
		break
	}
	d.err = d.text("file.actionUnavailable")
}

func (d *Desktop) completeFileAction(params ipc.Params, method, path string) {
	params.RepoID = d.selected
	d.task(func(ctx context.Context) (func(), error) {
		err := d.Client.Call(ctx, method, params, nil)
		return func() {
			d.path, d.result = path, d.text("file.actionDone")
		}, err
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
			if d.action == "clone" && d.clonedRepo != nil {
				d.cloneResult(c)
			} else {
				native.Column(c).Grow(1).FillWidth().Center().Gap(controlSpacing).Children(func() {
					d.resultHeading(c, emptyDiffArt, d.result, d.path)
					if d.offerCommitPush {
						native.Text(c, d.copy("推送本次提交？", "Push this commit?"))
						d.showCommitPushFailure(c)
					}
					native.Row(c).Gap(controlSpacing).Children(func() {
						if IconButton(c, d.text("close"), "close", !d.offerCommitPush).Disabled(d.busy).Clicked() && d.win != nil {
							d.win.Close()
						}
						if d.offerCommitPush && d.pushFailure == "" && activityButton(c, d.text("remote.push"), "push", true, d.remoteRunning == "push").Disabled(d.busy).Clicked() {
							d.remoteAction("push")
						}
					})
				})
			}
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
		d.showCommitPushFailure(c)
		if d.action == "log" && d.remoteNotice != "" {
			native.Text(c, d.remoteNotice).FillWidth().TextColor(t.TextMuted)
		}
		if d.err != "" && !d.settingsOpen {
			scrollAtPanelEdge(native.Scroll(c)).MinHeight(34).MaxHeight(132).Children(func() {
				native.Text(c, d.text("error")+": "+d.err).FillWidth().TextColor(t.Danger)
			})
		}
		if d.action == "clone" {
			if d.operation == cloneOperation {
				d.cloneProgress(c)
			} else {
				scrollAtPanelEdge(native.Scroll(c)).Grow(1).Children(func() { d.cloneView(c) })
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
		case "commit", "diff", "file-diff":
			d.workspace(c, t)
		case "log":
			if d.page == 0 {
				d.workspace(c, t)
			} else {
				d.showHistory(c, t)
			}
		}
	})
}

func (d *Desktop) resultHeading(c *native.Context, art func() *native.Bitmap, title, path string) {
	native.Image(c, art()).Width(128).Height(128).Fit(native.Contain)
	native.Text(c, title).Bold()
	native.Text(c, path).FillWidth().SingleLine().Ellipsis("…").TextAlign(native.Center).Tooltip(path)
}
