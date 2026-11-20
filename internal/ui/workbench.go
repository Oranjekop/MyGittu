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
	comparison                          app.BranchComparisonSummary
	comparisonPatch                     patchView
	comparisonCommits                   native.ListState
	comparisonFiles                     native.ListState
	comparisonFileRow                   int
	comparisonFile                      string
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
	const minimumPaneWidth = 294
	const buttonChrome = 2*8 + 6 + 14 // Horizontal padding, icon gap, and icon.
	buttonWidth := float32(minimumPaneWidth-2*panelSpacing-controlSpacing) / 2
	for _, key := range []string{"compare.useSelected", "branch.switch", "branch.merge", "branch.delete"} {
		width, _ := c.MeasureText(0, native.Span{Text: d.text(key)})
		buttonWidth = max(buttonWidth, width+buttonChrome)
	}
	paneWidth := 2*buttonWidth + 2*panelSpacing + controlSpacing
	native.Row(c).Grow(1).FillWidth().Gap(panelSpacing).Children(func() {
		native.Column(c).Width(paneWidth).FillHeight().Padding(panelSpacing).Gap(controlSpacing).Radius(8).Background(c.Theme().Background).Children(func() {
			native.Text(c, d.text("nav.branches")).Bold().TextColor(c.Theme().TextMuted).PaddingStart(controlSpacing)
			native.Divider(c)
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
			w.branchList.RoundRows = true
			list := scrollAtPanelEdge(native.List(c, &w.branchList, len(w.branches), func(i int) {
				branch := w.branches[i]
				name := strings.TrimPrefix(branch.Name, "refs/remotes/")
				native.Column(c).FillWidth().Padding(detailSpacing, controlSpacing).Gap(detailSpacing).Children(func() {
					native.Text(c, name).Bold().Ellipsis("…").Tooltip(branch.Name)
					if branch.Current {
						native.Text(c, d.text("branch.current")).FontSize(11)
					}
					if branch.Upstream != "" {
						native.Text(c, "↗ "+branch.Upstream).FontSize(11).Ellipsis("…")
					}
				})
			})).MinHeight(0).Clip().Disabled(d.busy)
			if len(w.branches) == 0 && !d.busy && d.err == "" {
				list.Height(0)
				emptyState(c, emptyPreviewArt, 88, d.copy("暂无分支", "No branches")).Grow(1)
			} else {
				list.Grow(1)
			}
			if row != previous && row >= 0 && row < len(w.branches) {
				selected = w.branches[row]
				w.branchSelected = selected.Name
			}
			native.Row(c).FillWidth().Gap(controlSpacing).Children(func() {
				if IconButton(c, d.text("compare.useSelected"), "diff", false).Width(buttonWidth).Center().Disabled(d.busy || selected.Name == "").Clicked() {
					w.right, w.compared = selected.Name, false
					for _, branch := range w.branches {
						if branch.Current {
							w.left = branch.Name
							break
						}
					}
				}
				d.branchAction(c, selected, buttonWidth, "switch")
			})
			native.Row(c).FillWidth().Gap(controlSpacing).Children(func() {
				d.branchAction(c, selected, buttonWidth, "merge")
				d.branchAction(c, selected, buttonWidth, "delete")
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

func (d *Desktop) branchAction(c *native.Context, branch app.BranchEntry, width float32, action string) {
	if IconButton(c, d.text("branch."+action), recoveryIcon(action), false).Width(width).Center().
		Disabled(d.busy || branch.Name == "" || branch.Current || branch.Remote).Clicked() {
		d.mutate("git.branch", ipc.Params{Action: action, Name: branch.Name})
	}
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
				var result app.BranchComparisonSummary
				err := d.Client.Call(ctx, "git.compare.summary", ipc.Params{RepoID: id, Name: left, Query: right}, &result)
				return func() {
					w.setComparison(result)
				}, err
			})
		}
	})
	if w.compared && len(w.comparison.Commits) == 0 && len(w.comparison.Paths) == 0 {
		emptyState(c, emptyDiffArt, 128, d.text("compare.same")).Grow(1)
	} else if w.compared {
		native.Segmented(c, &w.compareTab, d.copy("独有提交", "Unique commits"), d.text("compare.files"))
		if w.compareTab == 1 {
			d.showComparisonFiles(c)
		} else if len(w.comparison.Commits) == 0 {
			emptyState(c, emptyDiffArt, 128, d.copy("两个分支没有独有提交。", "Neither branch has unique commits.")).Grow(1)
		} else {
			roundedTable(c, &w.comparisonCommits, []native.TableColumn{
				{Title: d.copy("所属分支", "Branch"), Width: 145}, {Title: d.text("subject")}, {Title: d.text("author"), Width: 125}, {Title: d.text("date"), Width: dateColumnWidth},
			}, len(w.comparison.Commits), func(row, col int) {
				item := w.comparison.Commits[row]
				branch := w.left
				if item.Side == ">" {
					branch = w.right
				}
				value := []string{branch, item.Subject, item.Author, item.Date}[col]
				tooltip := item.Hash
				if col == 3 {
					value = displayDateTime(value)
					tooltip = value
				}
				native.Text(c, value).SingleLine().Ellipsis("…").Tooltip(tooltip)
			}).Grow(1)
			if w.comparison.More && IconButton(c, d.copy("加载更多提交", "Load more commits"), "down", false).Disabled(d.busy).Clicked() {
				d.loadMoreComparisonCommits()
			}
		}
	} else if !d.busy && d.err == "" {
		emptyState(c, emptyDiffArt, 128, d.copy("选择分支进行对比", "Select branches to compare")).Grow(1)
	}
}

func (w *workbench) setComparison(result app.BranchComparisonSummary) {
	w.comparison, w.compared = result, true
	w.comparisonCommits, w.comparisonFiles = native.ListState{}, native.ListState{}
	w.comparisonFileRow, w.comparisonFile, w.comparisonPatch = -1, "", patchView{}
}

func (d *Desktop) loadMoreComparisonCommits() {
	w := &d.tools
	params := ipc.Params{RepoID: d.selected, Name: w.comparison.Left, Query: w.comparison.Right, Offset: len(w.comparison.Commits)}
	d.task(func(ctx context.Context) (func(), error) {
		var page app.BranchComparisonPage
		err := d.Client.Call(ctx, "git.compare.page", params, &page)
		return func() {
			w.comparison.Commits = append(w.comparison.Commits, page.Commits...)
			w.comparison.More = page.More
		}, err
	})
}

func (d *Desktop) showComparisonFiles(c *native.Context) {
	w := &d.tools
	if len(w.comparison.Paths) == 0 {
		emptyState(c, emptyDiffArt, 128, d.copy("没有文件差异", "No file differences")).Grow(1)
		return
	}
	native.Row(c).Grow(1).FillWidth().Gap(panelSpacing).Children(func() {
		w.comparisonFiles.Selected = &w.comparisonFileRow
		roundedTable(c, &w.comparisonFiles, []native.TableColumn{{Title: d.copy("变更文件", "Changed files")}}, len(w.comparison.Paths), func(row, _ int) {
			name := w.comparison.Paths[row]
			native.Text(c, name).SingleLine().Ellipsis("…").Tooltip(name)
		}).Width(220 + panelSpacing).FillHeight().Disabled(d.busy)
		if w.comparisonFileRow >= 0 && w.comparisonFileRow < len(w.comparison.Paths) {
			name := w.comparison.Paths[w.comparisonFileRow]
			if name != w.comparisonFile && !d.busy {
				params := ipc.Params{RepoID: d.selected, Name: w.comparison.Left, Query: w.comparison.Right, Path: name}
				w.comparisonFile, w.comparisonPatch = name, patchView{}
				d.task(func(ctx context.Context) (func(), error) {
					var patch string
					err := d.Client.Call(ctx, "git.compare.file", params, &patch)
					if err != nil {
						return func() {
							w.comparisonFileRow, w.comparisonFile = -1, ""
							d.err = err.Error()
						}, nil
					}
					files, err := editor.ParsePatch(patch)
					return func() {
						if err != nil {
							w.comparisonFileRow, w.comparisonFile = -1, ""
							d.err = err.Error()
							return
						}
						w.comparisonPatch = patchView{lines: d.patchRows(files, false)}
					}, nil
				})
			}
		}
		if len(w.comparisonPatch.lines) > 0 {
			d.showPatch(c, &w.comparisonPatch).Grow(1).FillHeight()
		}
	})
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
