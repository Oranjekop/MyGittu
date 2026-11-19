package ui

import (
	"context"
	"path/filepath"
	"slices"
	"strings"

	native "github.com/egoist/mygo/ui"
	"mygittu/internal/app"
	git "mygittu/internal/git"
	"mygittu/internal/ipc"
	"mygittu/internal/repository"
	"mygittu/internal/status"
)

type repositoryTools struct {
	remoteOpen, treeOpen, confirmOpen, amendOpen bool
	remotes                                      []app.RemoteEntry
	branches                                     []app.BranchEntry
	trees                                        []app.Worktree
	remoteList, treeList                         native.ListState
	remoteRow, treeRow                           int
	name, url, branch, path, treeBranch          string
	question, detail, err                        string
	confirm                                      func()
	amendHash, amendMessage                      string
}

func (d *Desktop) repositoryToolButtons(c *native.Context) {
	native.Row(c).Gap(controlSpacing).Children(func() {
		if IconButton(c, d.copy("远程仓库", "Remotes"), "fetch", false).Disabled(d.busy).Clicked() {
			d.loadTools(true)
		}
		if IconButton(c, d.copy("工作树", "Worktrees"), "open", false).Disabled(d.busy).Clicked() {
			d.loadTools(false)
		}
	})
}

func (d *Desktop) loadTools(remote bool) {
	id := d.selected
	d.task(func(ctx context.Context) (func(), error) {
		var remotes []app.RemoteEntry
		var trees []app.Worktree
		var branches []app.BranchEntry
		var err error
		if remote {
			err = d.Client.Call(ctx, "git.remotes", ipc.Params{RepoID: id}, &remotes)
			if err == nil {
				err = d.Client.Call(ctx, "git.branch.entries", ipc.Params{RepoID: id}, &branches)
			}
		} else {
			err = d.Client.Call(ctx, "git.worktrees", ipc.Params{RepoID: id}, &trees)
		}
		return func() {
			w := &d.repositoryTools
			w.err = ""
			if remote {
				w.remotes, w.remoteOpen, w.remoteRow, w.branches = remotes, true, -1, branches
				if w.branch == "" {
					for _, branch := range branches {
						if branch.Current {
							w.branch = branch.Name
							break
						}
					}
				}
			} else {
				w.trees, w.treeOpen, w.treeRow = trees, true, -1
			}
		}, err
	})
}

func (d *Desktop) editTool(remote bool, method string, p ipc.Params) {
	p.RepoID = d.selected
	d.repositoryTools.err = ""
	d.task(func(ctx context.Context) (func(), error) {
		err := d.Client.Call(ctx, method, p, nil)
		var repos []repository.Info
		if err == nil && !remote && p.Action == "remove" {
			_ = d.Client.Call(ctx, "repo.list", ipc.Params{}, &repos)
		}
		return func() {
			if err != nil {
				d.repositoryTools.err = err.Error()
				return
			}
			if repos != nil {
				d.repos = repos
			}
			d.loadTools(remote)
		}, nil
	})
}

func (d *Desktop) askAction(question, detail string, confirm func()) {
	w := &d.repositoryTools
	w.question, w.detail, w.confirm, w.confirmOpen = question, detail, confirm, true
}

func (d *Desktop) showRepositoryTools(c *native.Context) {
	w := &d.repositoryTools
	native.Modal(c, &w.remoteOpen, func() { d.remoteManager(c, w) })
	native.Modal(c, &w.treeOpen, func() { d.worktreeManager(c, w) })
	native.Modal(c, &w.amendOpen, func() {
		native.Column(c).Width(470).Gap(panelSpacing).Children(func() {
			native.Text(c, d.copy("修正最近提交", "Amend latest commit")).Bold()
			native.Text(c, d.copy("此操作会改写最近一次提交，并包含所有已暂存的改动。", "This rewrites the latest commit and includes all staged changes.")).FillWidth()
			native.TextArea(c, &w.amendMessage).Height(110).FillWidth().Disabled(d.busy)
			native.Row(c).Gap(controlSpacing).Children(func() {
				if IconButton(c, d.copy("确认修正", "Amend commit"), "commit", true).Disabled(d.busy || strings.TrimSpace(w.amendMessage) == "").Clicked() {
					w.amendOpen = false
					d.mutate("git.amend", ipc.Params{Hash: w.amendHash, Message: w.amendMessage})
				}
				d.toolClose(c, &w.amendOpen)
			})
		})
	})
	native.Modal(c, &w.confirmOpen, func() {
		native.Column(c).Width(450).Gap(panelSpacing).Children(func() {
			native.Text(c, w.question).Bold().FillWidth()
			if w.detail != "" {
				native.Text(c, w.detail).FillWidth()
			}
			native.Row(c).Gap(controlSpacing).Children(func() {
				if IconButton(c, d.copy("确认", "Confirm"), "commit", true).Disabled(d.busy).Clicked() {
					w.confirmOpen = false
					apply := w.confirm
					w.confirm = nil
					if apply != nil {
						apply()
					}
				}
				d.toolClose(c, &w.confirmOpen)
			})
		})
	})
}

func (d *Desktop) remoteManager(c *native.Context, w *repositoryTools) {
	native.Column(c).Width(590).Gap(controlSpacing).Children(func() {
		native.Text(c, d.copy("远程仓库", "Remotes")).Bold()
		d.toolError(c, w)
		previous := w.remoteRow
		w.remoteList.Selected = &w.remoteRow
		table := roundedTable(c, &w.remoteList, []native.TableColumn{{Title: d.copy("名称", "Name"), Width: 105}, {Title: "URL"}}, len(w.remotes), func(row, col int) {
			item := w.remotes[row]
			if col == 0 {
				native.Text(c, item.Name)
			} else {
				native.Text(c, item.URL).SingleLine().Ellipsis("…").Tooltip(item.URL)
			}
		}).Height(130).FillWidth().Disabled(d.busy)
		if len(w.remotes) == 0 && !d.busy && w.err == "" {
			table.Height(28)
			emptyState(c, emptyPreviewArt, 64, d.copy("暂无远程仓库", "No remotes"))
		}
		if w.remoteRow >= 0 && w.remoteRow < len(w.remotes) && previous != w.remoteRow {
			w.name, w.url = w.remotes[w.remoteRow].Name, w.remotes[w.remoteRow].URL
		}
		native.Row(c).Gap(controlSpacing).Children(func() {
			native.TextInput(c, &w.name).Placeholder(d.copy("远程名称", "Remote name")).Width(120).Disabled(d.busy)
			native.TextInput(c, &w.url).Placeholder(d.copy("远程地址", "Remote URL")).Grow(1).Disabled(d.busy)
		})
		name, url := strings.TrimSpace(w.name), strings.TrimSpace(w.url)
		exists := slices.ContainsFunc(w.remotes, func(r app.RemoteEntry) bool { return r.Name == name })
		run := func(action, value string) {
			d.editTool(true, "git.remote.configure", ipc.Params{Action: action, Name: name, URL: value})
		}
		native.Row(c).Gap(controlSpacing).Wrap().Children(func() {
			if IconButton(c, d.copy("新增", "Add"), "plus", false).Disabled(d.busy || exists || name == "" || url == "").Clicked() {
				run("add", url)
			}
			if IconButton(c, d.copy("更新地址", "Update URL"), "save", false).Disabled(d.busy || !exists || url == "").Clicked() {
				run("set-url", url)
			}
			if IconButton(c, d.copy("获取", "Fetch"), "fetch", false).Disabled(d.busy || !exists).Clicked() {
				run("fetch", "")
			}
			if IconButton(c, d.copy("移除远程", "Remove remote"), "remove", false).Disabled(d.busy || !exists).Clicked() {
				d.askAction(d.copy("移除此远程仓库配置？", "Remove this remote configuration?"), name, func() { run("remove", "") })
			}
		})
		native.TextInput(c, &w.branch).Placeholder(d.copy("远程分支名，例如 main", "Remote branch, e.g. main")).FillWidth().Disabled(d.busy)
		prefix := "refs/remotes/" + name + "/"
		var branches []string
		for _, ref := range w.branches {
			if ref.Remote && strings.HasPrefix(ref.Name, prefix) {
				branches = append(branches, strings.TrimPrefix(ref.Name, prefix))
			}
		}
		if len(branches) > 0 {
			var picked string
			label := d.copy("选择已获取的远程分支", "Select a fetched remote branch")
			native.Select(c, &picked, branches).Label(label).Placeholder(label).FillWidth().Disabled(d.busy)
			if picked != "" {
				w.branch = picked
			}
		}
		branch := strings.TrimSpace(w.branch)
		native.Row(c).Gap(controlSpacing).Wrap().Children(func() {
			for _, b := range []struct{ action, icon, zh, en string }{
				{"pull", "pull", "拉取（仅快进）", "Pull (fast-forward only)"},
				{"track", "branch", "设置跟踪", "Set upstream"},
				{"push", "push", "推送并跟踪", "Push and track"},
			} {
				if IconButton(c, d.copy(b.zh, b.en), b.icon, false).Disabled(d.busy || !exists || branch == "").Clicked() {
					run(b.action, branch)
				}
			}
			d.toolClose(c, &w.remoteOpen)
		})
	})
}

func (d *Desktop) worktreeManager(c *native.Context, w *repositoryTools) {
	native.Column(c).Width(590).Gap(controlSpacing).Children(func() {
		native.Text(c, d.copy("工作树", "Worktrees")).Bold()
		d.toolError(c, w)
		w.treeList.Selected = &w.treeRow
		table := roundedTable(c, &w.treeList, []native.TableColumn{{Title: d.copy("分支", "Branch"), Width: 130}, {Title: d.copy("目录", "Folder")}}, len(w.trees), func(row, col int) {
			item := w.trees[row]
			if col == 0 {
				native.Text(c, item.Branch)
			} else {
				native.Text(c, item.Path).SingleLine().Ellipsis("…").Tooltip(item.Path)
			}
		}).Height(160).FillWidth().Disabled(d.busy)
		if len(w.trees) == 0 && !d.busy && w.err == "" {
			table.Height(28)
			emptyState(c, emptyPreviewArt, 64, d.copy("暂无工作树", "No worktrees"))
		}
		if w.treeRow >= 0 && w.treeRow < len(w.trees) {
			item := w.trees[w.treeRow]
			native.Row(c).Gap(controlSpacing).Children(func() {
				if IconButton(c, d.copy("打开工作树", "Open worktree"), "open", false).Disabled(d.busy || item.Prunable).Clicked() {
					w.treeOpen = false
					d.open(item.Path)
				}
				if IconButton(c, d.copy("移除工作树", "Remove worktree"), "remove", false).Disabled(d.busy || item.Primary || item.Locked || item.Prunable || filepath.Clean(item.Path) == filepath.Clean(d.path)).Clicked() {
					d.askAction(d.copy("移除此工作树及其目录？", "Remove this worktree and its directory?"), item.Path, func() { d.editTool(false, "git.worktree.edit", ipc.Params{Action: "remove", Path: item.Path}) })
				}
			})
		}
		native.TextInput(c, &w.treeBranch).Placeholder(d.copy("现有本地分支", "Existing local branch")).FillWidth().Disabled(d.busy)
		native.TextInput(c, &w.path).Placeholder(d.copy("工作树的绝对路径", "Absolute worktree path")).FillWidth().Disabled(d.busy)
		native.Row(c).Gap(controlSpacing).Children(func() {
			if IconButton(c, d.copy("创建工作树", "Create worktree"), "plus", true).Disabled(d.busy || !filepath.IsAbs(strings.TrimSpace(w.path)) || strings.TrimSpace(w.treeBranch) == "").Clicked() {
				d.editTool(false, "git.worktree.edit", ipc.Params{Action: "add", Path: strings.TrimSpace(w.path), Name: strings.TrimSpace(w.treeBranch)})
			}
			d.toolClose(c, &w.treeOpen)
		})
	})
}
func (d *Desktop) toolError(c *native.Context, w *repositoryTools) {
	if w.err != "" {
		native.Text(c, w.err).FillWidth().TextColor(c.Theme().Danger)
	}
}
func (d *Desktop) toolClose(c *native.Context, open *bool) {
	if IconButton(c, d.text("close"), "close", false).Clicked() {
		*open = false
	}
}

func (d *Desktop) prepareAmend() {
	id := d.selected
	d.task(func(ctx context.Context) (func(), error) {
		var target app.AmendTarget
		err := d.Client.Call(ctx, "git.amend.target", ipc.Params{RepoID: id}, &target)
		return func() {
			d.repositoryTools.amendHash, d.repositoryTools.amendMessage, d.repositoryTools.amendOpen = target.Hash, target.Message, true
		}, err
	})
}
func (d *Desktop) cherryControls(c *native.Context, files []git.FileStatus) {
	native.Row(c).Gap(controlSpacing).Wrap().Children(func() {
		conflict := slices.ContainsFunc(files, func(f git.FileStatus) bool { return f.State == git.Conflicted })
		if IconButton(c, d.copy("继续拣选", "Continue cherry-pick"), "right", true).Disabled(d.busy || conflict).Clicked() {
			d.runCherry("continue", "")
		}
		if IconButton(c, d.copy("中止拣选", "Abort cherry-pick"), "close", false).Disabled(d.busy).Clicked() {
			d.askAction(d.copy("中止拣选并恢复到操作前的状态？", "Abort cherry-pick and restore its starting state?"), "", func() { d.runCherry("abort", "") })
		}
	})
}
func (d *Desktop) runCherry(action, hash string) {
	id := d.selected
	d.task(func(ctx context.Context) (func(), error) {
		err := d.Client.Call(ctx, "git.cherry-pick", ipc.Params{RepoID: id, Action: action, Hash: hash}, nil)
		var snapshot status.Snapshot
		statusErr := d.Client.Call(ctx, "status.refresh", ipc.Params{RepoID: id}, &snapshot)
		return func() {
			if statusErr == nil && (err == nil || snapshot.CherryPicking) {
				d.page = 0
				d.applyWorkspaceStatus(snapshot)
			}
			if err != nil && (action != "start" || !snapshot.CherryPicking) {
				d.err = err.Error()
			}
		}, nil
	})
}

func (d *Desktop) restoreFileButton(c *native.Context, file git.FileStatus) {
	if d.staged || file.Worktree == git.Clean || file.State == git.Conflicted || file.Index == git.Deleted {
		return
	}
	label, question, action := d.copy("恢复文件", "Restore file"), d.copy("将文件恢复为暂存区版本并丢弃未暂存改动？", "Restore the index version and discard unstaged changes?"), "tracked"
	if file.Worktree == git.Untracked {
		label, question, action = d.copy("删除未跟踪文件", "Delete untracked file"), d.copy("永久删除此未跟踪文件？", "Permanently delete this untracked file?"), "untracked"
	}
	if IconButton(c, label, "undo", false).Disabled(d.busy).Clicked() {
		params := ipc.Params{RepoID: d.selected, Path: file.Path, Action: action}
		d.task(func(ctx context.Context) (func(), error) {
			var token string
			err := d.Client.Call(ctx, "git.restore.preview", params, &token)
			return func() {
				params.Hash = token
				d.askAction(question, file.Path, func() { d.mutate("git.restore", params) })
			}, err
		})
	}
}
