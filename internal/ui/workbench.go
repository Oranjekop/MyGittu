package ui

import (
	"context"
	"strings"

	native "github.com/egoist/mygo/ui"
	"mygittu/internal/app"
	git "mygittu/internal/git"
	"mygittu/internal/ipc"
)

type workbench struct {
	conflictOpen, fileOpen, confirmOpen bool
	conflict                            app.Conflict
	conflictPath, resolution            string
	filePath, fileResult                string
	fileCommits                         []app.Commit
	fileRow                             int
	fileList                            native.ListState
	blameList                           native.ListState
	blameRows                           []string
	blameRow                            int
	recoveryAction, recoveryHash        string
	left, right                         string
	branchTargets                       []string
	branches                            []app.BranchEntry
	branchSelected                      string
	branchList                          native.ListState
	compareTab                          int
	compared                            bool
	comparison                          app.BranchComparison
	batch                               map[string]bool
}

func (d *Desktop) batchPaths(files []git.FileStatus) []string {
	add := func(paths []string, file git.FileStatus) []string {
		paths = append(paths, file.Path)
		if file.OriginalPath != "" {
			paths = append(paths, file.OriginalPath)
		}
		return paths
	}
	var paths []string
	if len(d.tools.batch) > 0 {
		for _, file := range files {
			if d.tools.batch[file.Path] {
				paths = add(paths, file)
			}
		}
		return paths
	}
	if d.row >= 0 && d.row < len(files) {
		return add(nil, files[d.row])
	}
	return nil
}

func (d *Desktop) showBatchActions(c *native.Context, files []git.FileStatus) {
	if len(d.tools.batch) == 0 {
		return
	}
	count := 0
	for _, file := range files {
		if d.tools.batch[file.Path] {
			count++
		}
	}
	native.Column(c).FillWidth().Padding(controlSpacing).Gap(controlSpacing).Radius(6).Background(c.Theme().Surface).Children(func() {
		native.Textf(c, d.text("batch.count"), count).Bold()
		native.Row(c).Gap(controlSpacing).Wrap().Children(func() {
			for _, op := range []struct{ label, method string }{{"batch.stage", "git.stage"}, {"batch.unstage", "git.unstage"}} {
				if native.Button(c, d.text(op.label)).Padding(5, 7).Disabled(d.busy || count == 0).Clicked() {
					d.mutate(op.method, ipc.Params{Paths: d.batchPaths(files)})
				}
			}
			if native.Button(c, d.text("batch.clear")).Padding(5, 7).Disabled(d.busy).Clicked() {
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
				for _, side := range []struct{ label, text string }{
					{"conflict.ours", w.conflict.Ours}, {"conflict.theirs", w.conflict.Theirs},
				} {
					native.Column(c).Grow(1).MinWidth(0).Gap(controlSpacing).Children(func() {
						native.TextArea(c, &side.text).ReadOnly(true).Height(135).FillWidth()
						if native.Button(c, d.text(side.label)).Clicked() {
							w.resolution = side.text
						}
					})
				}
			})
			native.Text(c, d.text("conflict.result")).Bold()
			native.TextArea(c, &w.resolution).Height(140).FillWidth()
			native.Row(c).Gap(controlSpacing).Children(func() {
				if native.PrimaryButton(c, d.text("conflict.save")).Disabled(d.busy).Clicked() {
					id, path, token, content := d.selected, w.conflictPath, w.conflict.Token, w.resolution
					d.task(func(ctx context.Context) (func(), error) {
						err := d.Client.Call(ctx, "git.resolve", ipc.Params{RepoID: id, Path: path, Hash: token, Message: content}, nil)
						return func() { w.conflictOpen = false; d.refresh() }, err
					})
				}
				if native.Button(c, d.text("close")).Clicked() {
					w.conflictOpen = false
				}
			})
			if d.err != "" {
				native.Text(c, d.err).TextColor(c.Theme().Danger)
			}
		})
	})
}

func (d *Desktop) openFileHistory(path string) {
	id := d.selected
	d.task(func(ctx context.Context) (func(), error) {
		var list []app.Commit
		err := d.Client.Call(ctx, "git.file.log", ipc.Params{RepoID: id, Path: path, Limit: 100}, &list)
		return func() {
			w := &d.tools
			w.filePath, w.fileCommits, w.fileRow, w.fileResult, w.fileOpen = path, list, -1, "", true
			w.blameRows, w.blameRow = nil, -1
		}, err
	})
}

func (d *Desktop) showFileHistory(c *native.Context) {
	w := &d.tools
	native.Modal(c, &w.fileOpen, func() {
		native.Column(c).Width(730).Gap(controlSpacing).Children(func() {
			native.Text(c, w.filePath).Bold()
			w.fileList.Selected = &w.fileRow
			native.Table(c, &w.fileList, []native.TableColumn{{Title: d.text("file.history")}}, len(w.fileCommits), func(row, _ int) {
				commit := w.fileCommits[row]
				native.Text(c, commit.Hash[:min(len(commit.Hash), 8)]+"  "+commit.Subject+" · "+commit.Author)
			}).Height(145).FillWidth().Disabled(d.busy)
			show := func(action, hash string) {
				id, path := d.selected, w.filePath
				d.task(func(ctx context.Context) (func(), error) {
					var result string
					err := d.Client.Call(ctx, "git.file.insight", ipc.Params{RepoID: id, Path: path, Action: action, Hash: hash}, &result)
					return func() {
						w.blameRows, w.blameRow = nil, -1
						if action == "blame" && result != "" {
							w.blameRows = strings.Split(strings.TrimRight(result, "\r\n"), "\n")
						}
						w.fileResult = result
					}, err
				})
			}
			native.Row(c).Gap(controlSpacing).Children(func() {
				valid := w.fileRow >= 0 && w.fileRow < len(w.fileCommits)
				if native.Button(c, d.text("file.patch")).Disabled(d.busy || !valid).Clicked() && valid {
					show("patch", w.fileCommits[w.fileRow].Hash)
				}
				if native.Button(c, d.text("file.blame")).Disabled(d.busy).Clicked() {
					show("blame", "")
				}
				if native.Button(c, d.text("close")).Clicked() {
					w.fileOpen = false
				}
			})
			if d.err != "" {
				native.Text(c, d.err).TextColor(c.Theme().Danger)
			}
			if len(w.blameRows) > 0 {
				w.blameList.Selected = &w.blameRow
				native.Table(c, &w.blameList, []native.TableColumn{{Title: d.text("file.blame")}}, len(w.blameRows), func(row, _ int) {
					hash, line, _ := strings.Cut(w.blameRows[row], " ")
					native.Text(c, hash[:min(len(hash), 8)]+" "+line).Tooltip(w.blameRows[row])
				}).Height(230).FillWidth().Disabled(d.busy)
				if w.blameRow >= 0 && w.blameRow < len(w.blameRows) {
					native.Text(c, w.blameRows[w.blameRow]).FillWidth()
				}
			} else {
				native.TextArea(c, &w.fileResult).Height(230).FillWidth().ReadOnly(true)
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
				if native.PrimaryButton(c, d.text("recover."+w.recoveryAction)).Disabled(d.busy).Clicked() {
					w.confirmOpen = false
					d.mutate("git.recover", ipc.Params{Action: w.recoveryAction, Hash: w.recoveryHash})
				}
				if native.Button(c, d.text("close")).Clicked() {
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
				if native.Button(c, d.text("compare.useSelected")).Disabled(d.busy || selected.Name == "").Clicked() {
					w.right, w.compared = selected.Name, false
					for _, branch := range w.branches {
						if branch.Current {
							w.left = branch.Name
							break
						}
					}
				}
				for _, op := range []struct{ label, action string }{{"branch.switch", "switch"}, {"branch.merge", "merge"}, {"branch.delete", "delete"}} {
					if native.Button(c, d.text(op.label)).Disabled(d.busy || selected.Name == "" || selected.Current || selected.Remote).Clicked() {
						d.mutate("git.branch", ipc.Params{Action: op.action, Name: selected.Name})
					}
				}
			})
			native.Row(c).Gap(controlSpacing).Children(func() {
				native.TextInput(c, &d.branch).Placeholder(d.text("branch.name")).Grow(1).MinWidth(0).Disabled(d.busy)
				if native.Button(c, d.text("branch.create")).Disabled(d.busy || strings.TrimSpace(d.branch) == "").Clicked() {
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
		if native.Button(c, d.text("compare.run")).Disabled(d.busy || w.left == "" || w.right == "").Clicked() {
			id, left, right := d.selected, w.left, w.right
			d.task(func(ctx context.Context) (func(), error) {
				var result app.BranchComparison
				err := d.Client.Call(ctx, "git.compare", ipc.Params{RepoID: id, Name: left, Query: right}, &result)
				return func() { w.comparison, w.compared = result, true }, err
			})
		}
	})
	if w.compared && w.comparison.Graph == "" && w.comparison.Files == "" {
		native.Text(c, d.text("compare.same"))
	} else if w.compared {
		native.Segmented(c, &w.compareTab, d.text("compare.graph"), d.text("compare.files"))
		content := &w.comparison.Graph
		if w.compareTab == 1 {
			content = &w.comparison.Files
		}
		native.TextArea(c, content).Grow(1).FillWidth().ReadOnly(true)
	}
}
