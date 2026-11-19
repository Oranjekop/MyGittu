package ui

import (
	"context"
	"strings"

	native "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"mygittu/internal/app"
	git "mygittu/internal/git"
	"mygittu/internal/ipc"
)

type workbench struct {
	conflictOpen, fileOpen, confirmOpen bool
	conflict                            app.Conflict
	conflictPath, resolution            string
	filePath, fileLoaded, fileShown     string
	filePatch                           patchView
	fileCommits                         []app.Commit
	fileRow                             int
	fileList                            native.ListState
	blameList                           native.ListState
	blameRows                           []app.BlameLine
	fileMode                            int
	blameLoaded                         bool
	recoveryAction, recoveryHash        string
	left, right                         string
	branchTargets                       []string
	branches                            []app.BranchEntry
	branchSelected                      string
	branchList                          native.ListState
	compareTab                          int
	compared                            bool
	comparison                          app.BranchComparison
	comparisonPatch                     patchView
	comparisonCommits                   native.ListState
	batch                               map[string]bool
}

func stagingPaths(paths []string, file git.FileStatus, stage bool) []string {
	if file.State == git.Conflicted {
		return paths
	}
	if stage && (file.Worktree == git.Clean || file.Worktree == "") {
		return paths
	}
	if !stage && (file.Index == git.Clean || file.Index == "") {
		return paths
	}
	paths = append(paths, file.Path)
	if file.OriginalPath != "" {
		paths = append(paths, file.OriginalPath)
	}
	return paths
}

func (d *Desktop) stagingSelection(files []git.FileStatus) (stage, unstage []string, count int) {
	for i, file := range files {
		if !d.tools.batch[file.Path] && (len(d.tools.batch) != 0 || d.row != i) {
			continue
		}
		count++
		stage = stagingPaths(stage, file, true)
		unstage = stagingPaths(unstage, file, false)
		if len(d.tools.batch) == 0 {
			break
		}
	}
	return stage, unstage, count
}
func (d *Desktop) showStagingActions(c *native.Context, files []git.FileStatus) {
	stage, unstage, count := d.stagingSelection(files)
	if count == 0 {
		return
	}
	batch := len(d.tools.batch) > 0
	stageLabel, unstageLabel := "stage", "unstage"
	if batch {
		stageLabel, unstageLabel = "batch.stage", "batch.unstage"
	}
	native.Column(c).FillWidth().Padding(controlSpacing).Gap(detailSpacing).Radius(6).Background(c.Theme().Surface).Children(func() {
		if batch {
			native.Textf(c, d.text("batch.count"), count).Bold()
		}
		native.Row(c).Gap(detailSpacing).Wrap().Children(func() {
			if IconButton(c, d.text(stageLabel), "stage", false).Gap(3).Padding(0, 5).Disabled(d.busy || len(stage) == 0).Clicked() {
				d.mutate("git.stage", ipc.Params{Paths: stage})
			}
			if IconButton(c, d.text(unstageLabel), "unstage", false).Gap(3).Padding(0, 5).Disabled(d.busy || len(unstage) == 0).Clicked() {
				d.mutate("git.unstage", ipc.Params{Paths: unstage})
			}
			if batch && IconButton(c, d.text("batch.clear"), "clear", false).Gap(3).Padding(0, 5).Disabled(d.busy).Clicked() {
				clear(d.tools.batch)
			}
		})
	})
}
func (d *Desktop) loadConflict(path string) {
	id := d.selected
	d.task(func(ctx context.Context) (func(), error) {
		var info app.Conflict
		err := d.Client.Call(ctx, "git.conflict", ipc.Params{RepoID: id, Path: path}, &info)
		return func() {
			d.tools.conflict, d.tools.conflictPath, d.tools.resolution, d.tools.conflictOpen = info, path, info.Working, true
		}, err
	})
}

func (d *Desktop) showConflict(c *native.Context) {
	w := &d.tools
	native.Modal(c, &w.conflictOpen, func() {
		native.Column(c).Width(730).Gap(controlSpacing).Children(func() {
			native.Text(c, w.conflictPath).Bold()
			native.Row(c).FillWidth().Gap(controlSpacing).Children(func() {
				for _, side := range []struct{ label, text, icon string }{
					{"conflict.ours", w.conflict.Ours, "left"}, {"conflict.theirs", w.conflict.Theirs, "right"},
				} {
					native.Column(c).Grow(1).MinWidth(0).Gap(controlSpacing).Children(func() {
						native.TextArea(c, &side.text).ReadOnly(true).Height(135).FillWidth()
						if IconButton(c, d.text(side.label), side.icon, false).Clicked() {
							w.resolution = side.text
						}
					})
				}
			})
			native.Text(c, d.text("conflict.result")).Bold()
			native.TextArea(c, &w.resolution).Height(140).FillWidth()
			native.Row(c).Gap(controlSpacing).Children(func() {
				if IconButton(c, d.text("conflict.save"), "save", true).Disabled(d.busy).Clicked() {
					id, path, token, content := d.selected, w.conflictPath, w.conflict.Token, w.resolution
					d.task(func(ctx context.Context) (func(), error) {
						err := d.Client.Call(ctx, "git.resolve", ipc.Params{RepoID: id, Path: path, Hash: token, Message: content}, nil)
						return func() { w.conflictOpen = false; d.refresh() }, err
					})
				}
				if IconButton(c, d.text("close"), "close", false).Clicked() {
					w.conflictOpen = false
				}
			})
			if d.err != "" {
				native.Text(c, d.err).TextColor(c.Theme().Danger)
			}
		})
	})
}

func (d *Desktop) confirmRecovery(action, hash string) {
	d.tools.recoveryAction, d.tools.recoveryHash, d.tools.confirmOpen = action, hash, true
}

func (d *Desktop) showRecovery(c *native.Context) {
	w := &d.tools
	native.Modal(c, &w.confirmOpen, func() {
		native.Column(c).Width(420).Gap(panelSpacing).Children(func() {
			native.Text(c, d.text("recover."+w.recoveryAction+".help")).FillWidth()
			if w.recoveryHash != "" {
				native.Text(c, w.recoveryHash).FontSize(11)
			}
			native.Row(c).Gap(controlSpacing).Children(func() {
				if IconButton(c, d.text("recover."+w.recoveryAction), recoveryIcon(w.recoveryAction), true).Disabled(d.busy).Clicked() {
					w.confirmOpen = false
					d.mutate("git.recover", ipc.Params{Action: w.recoveryAction, Hash: w.recoveryHash})
				}
				if IconButton(c, d.text("close"), "close", false).Clicked() {
					w.confirmOpen = false
				}
			})
		})
	})
}

func (d *Desktop) showBranches(c *native.Context) {
	w := &d.tools
	native.Row(c).Grow(1).FillWidth().Gap(panelSpacing).Children(func() {
		native.Column(c).Width(294).FillHeight().Padding(panelSpacing).Gap(controlSpacing).Radius(8).Background(c.Theme().Background).Children(func() {
			row := -1
			var selected app.BranchEntry
			for i, branch := range w.branches {
				if branch.Name == w.branchSelected {
					row, selected = i, branch
					break
				}
			}
			previous := row
			w.branchList.Selected = &row
			native.Table(c, &w.branchList, []native.TableColumn{{Title: d.text("nav.branches")}}, len(w.branches), func(i, _ int) {
				branch := w.branches[i]
				name := strings.TrimPrefix(branch.Name, "refs/remotes/")
				native.Column(c).Gap(detailSpacing).Children(func() {
					native.Text(c, name).Bold().Ellipsis("…").Tooltip(branch.Name)
					if branch.Current {
						native.Text(c, d.text("branch.current")).FontSize(11)
					}
					if branch.Upstream != "" {
						native.Text(c, "↗ "+branch.Upstream).FontSize(11).Ellipsis("…")
					}
				})
			}).Grow(1).FillWidth().Disabled(d.busy)
			if row != previous && row >= 0 && row < len(w.branches) {
				selected = w.branches[row]
				w.branchSelected = selected.Name
			}
			native.Row(c).Gap(controlSpacing).Wrap().Children(func() {
				if IconButton(c, d.text("compare.useSelected"), "diff", false).Disabled(d.busy || selected.Name == "").Clicked() {
					w.right, w.compared = selected.Name, false
					for _, branch := range w.branches {
						if branch.Current {
							w.left = branch.Name
							break
						}
					}
				}
				for _, op := range []struct{ label, action string }{{"branch.switch", "switch"}, {"branch.merge", "merge"}, {"branch.delete", "delete"}} {
					if IconButton(c, d.text(op.label), recoveryIcon(op.action), false).Disabled(d.busy || selected.Name == "" || selected.Current || selected.Remote).Clicked() {
						d.mutate("git.branch", ipc.Params{Action: op.action, Name: selected.Name})
					}
				}
			})
			native.Row(c).Gap(controlSpacing).Children(func() {
				native.TextInput(c, &d.branch).Placeholder(d.text("branch.name")).Grow(1).MinWidth(0).Disabled(d.busy)
				if IconButton(c, d.text("branch.create"), "branch", false).Disabled(d.busy || strings.TrimSpace(d.branch) == "").Clicked() {
					d.mutate("git.branch", ipc.Params{Action: "create", Name: strings.TrimSpace(d.branch)})
				}
			})
		})
		native.Column(c).Grow(1).MinWidth(0).FillHeight().Padding(panelSpacing).Gap(controlSpacing).Radius(8).Background(c.Theme().Background).Children(func() {
			d.showBranchComparison(c)
		})
	})
}

func (d *Desktop) showBranchComparison(c *native.Context) {
	w := &d.tools
	native.Row(c).Gap(controlSpacing).Wrap().Children(func() {
		leftBefore, rightBefore := w.left, w.right
		native.Select(c, &w.left, w.branchTargets).Grow(1).Label(d.text("compare.left")).Disabled(d.busy)
		native.Select(c, &w.right, w.branchTargets).Grow(1).Label(d.text("compare.right")).Disabled(d.busy)
		if w.left != leftBefore || w.right != rightBefore {
			w.compared = false
		}
		if IconButton(c, d.text("compare.run"), "diff", false).Disabled(d.busy || w.left == "" || w.right == "").Clicked() {
			id, left, right := d.selected, w.left, w.right
			d.task(func(ctx context.Context) (func(), error) {
				var result app.BranchComparison
				err := d.Client.Call(ctx, "git.compare", ipc.Params{RepoID: id, Name: left, Query: right}, &result)
				return func() {
					w.comparison, w.compared = result, true
					files, _ := editor.ParsePatch(result.Files)
					w.comparisonPatch = patchView{lines: d.patchRows(files, true)}
					w.comparisonCommits = native.ListState{}
				}, err
			})
		}
	})
	if w.compared && len(w.comparison.Commits) == 0 && w.comparison.Files == "" {
		native.Text(c, d.text("compare.same"))
	} else if w.compared {
		native.Segmented(c, &w.compareTab, d.copy("独有提交", "Unique commits"), d.text("compare.files"))
		if w.compareTab == 1 {
			if len(w.comparisonPatch.lines) == 0 {
				native.Text(c, d.text("diff.empty")).TextColor(c.Theme().TextMuted)
			} else {
				d.showPatch(c, &w.comparisonPatch).Grow(1)
			}
		} else if len(w.comparison.Commits) == 0 {
			native.Text(c, d.copy("两个分支没有独有提交。", "Neither branch has unique commits.")).TextColor(c.Theme().TextMuted)
		} else {
			native.Table(c, &w.comparisonCommits, []native.TableColumn{
				{Title: d.copy("所属分支", "Branch"), Width: 145}, {Title: d.text("subject")}, {Title: d.text("author"), Width: 125}, {Title: d.text("date"), Width: 110},
			}, len(w.comparison.Commits), func(row, col int) {
				item := w.comparison.Commits[row]
				branch := w.left
				if item.Side == ">" {
					branch = w.right
				}
				value := []string{branch, item.Subject, item.Author, commitDate(item.Date)}[col]
				native.Text(c, value).SingleLine().Ellipsis("…").Tooltip(item.Hash)
			}).Grow(1).FillWidth()
		}
	}
}

func recoveryIcon(action string) string {
	switch action {
	case "merge.continue":
		return "merge"
	case "delete":
		return "remove"
	default:
		return action
	}
}
