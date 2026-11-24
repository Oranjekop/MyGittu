package ui

import (
	"context"
	"path"
	"strings"
	"time"

	"github.com/egoist/mygo"
	native "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"mygittu/internal/app"
	"mygittu/internal/ipc"
)

const historyBatchSize = 100
const historyFileListWidth = 225

func (d *Desktop) invalidateHistoryLoad() {
	d.historyGeneration++
	d.historyLoading, d.historyLoadError = false, ""
}

func (d *Desktop) loadMoreHistory() {
	if d.Client == nil || d.busy || d.historyLoading || !d.historyMore || len(d.logs) == 0 {
		return
	}
	generation := d.historyGeneration
	last := d.logs[len(d.logs)-1].Hash
	params := ipc.Params{RepoID: d.selected, Offset: len(d.logs) - 1, Limit: historyBatchSize + 2,
		Query: d.historyQuery, AllBranches: d.historyAllBranches}
	d.historyLoading, d.historyLoadError = true, ""
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var more []app.Commit
		err := d.Client.Call(ctx, "git.log", params, &more)
		mygo.RunOnMain(func() {
			if generation != d.historyGeneration || d.win != nil && d.win.IsDestroyed() {
				return
			}
			d.historyLoading = false
			if err != nil {
				d.historyLoadError = err.Error()
			} else if len(more) == 0 || more[0].Hash != last {
				d.historyMore = false
				d.historyLoadError = d.copy("提交历史已变化，请刷新。", "Commit history changed. Refresh to continue.")
			} else {
				more = more[1:]
				d.historyMore = len(more) > historyBatchSize
				batch := more[:min(len(more), historyBatchSize)]
				d.logs = append(d.logs, batch...)
				if d.historyQuery != "" {
					d.historyGraph, d.historyGraphWidth = buildCommitGraph(d.logs, true)
				} else {
					rows, width := d.historyGraphBuilder.appendRows(batch, false)
					d.historyGraph = append(d.historyGraph, rows...)
					d.historyGraphWidth = width
				}
			}
			if d.win != nil {
				d.win.Invalidate()
			}
		})
	}()
}

func (d *Desktop) setHistoryPatch(patch string) {
	files, _ := editor.ParsePatch(patch)
	d.historyFiles, d.historyFileRow = files, 0
	d.historyFileList = native.ListState{}
	d.historyPatch = patchView{lines: d.patchRows(files, true)}
	d.historyFileDiff = editor.DiffState{}
}

func (d *Desktop) showCommitSummary(c *native.Context, commit app.Commit) {
	native.Scroll(c).ScrollbarStyle(native.ScrollbarStyle{TrackWidth: panelSpacing}).Grow(1).MinHeight(0).FillWidth().Children(func() {
		native.Column(c).FillWidth().PaddingEnd(panelSpacing).Gap(controlSpacing).Children(func() {
			native.Text(c, commit.Subject).Bold().FillWidth()
			native.Text(c, commit.Author+" · "+displayDateTime(commit.Date)).TextColor(c.Theme().TextMuted)
			subject, body, _ := strings.Cut(*d.historySummary, "\n")
			if subject != commit.Subject {
				body = *d.historySummary
			}
			if body = strings.TrimSpace(body); body != "" {
				native.Text(c, body).FillWidth()
			}
		})
	})
}

func (d *Desktop) showHistoryPatch(c *native.Context) {
	if len(d.historyFiles) == 0 {
		emptyState(c, emptyDiffArt, 88, d.copy("此提交没有文件差异。", "This commit has no file changes.")).Grow(1)
		return
	}
	native.Row(c).Grow(1).MinHeight(100).FillWidth().Gap(panelSpacing).Children(func() {
		d.historyFileList.Selected = &d.historyFileRow
		before := d.historyFileRow
		roundedTable(c, &d.historyFileList, []native.TableColumn{{Title: d.copy("变更文件", "Changed files")}}, len(d.historyFiles)+1, func(row, _ int) {
			if row == 0 {
				native.Text(c, d.text("diff.all"))
				return
			}
			file := d.historyFiles[row-1]
			name := patchFileName(file)
			filename := file.NewName
			if filename == "" {
				filename = file.OldName
			}
			icon := projectFileIcon(path.Base(filename))
			darkBackground := c.Theme().Dark != (row == d.historyFileRow)
			native.Row(c).FillWidth().MinWidth(0).Gap(detailSpacing).AlignItems(native.Center).Tooltip(name).Children(func() {
				native.Icon(c, icons[icon]).Size(14, 14).TextColor(fileIconColor(icon, darkBackground))
				native.Text(c, name).Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
			})
		}).Width(historyFileListWidth).FillHeight()
		if row := d.historyFileRow; row > 0 && row <= len(d.historyFiles) {
			file := d.historyFiles[row-1]
			if len(file.Hunks) > 0 && !file.Binary {
				if before != row || d.historyFileDiff.Old == nil {
					d.historyFileDiff = editor.DiffState{}
					d.historyFileDiff.SetPatch(file)
				}
				width, _ := c.Size()
				native.Column(c).Grow(1).MinWidth(0).FillHeight().Gap(controlSpacing).Children(func() {
					d.showCodeDiff(c, &d.historyFileDiff, width-historyFileListWidth-sidebarWidth-4*panelSpacing, false)
				})
				return
			}
			message := d.copy("文件属性或路径变更", "File properties or path changed")
			if file.Binary {
				message = d.text("diff.binary")
			}
			emptyState(c, emptyDiffArt, 88, message).Grow(1).FillHeight()
			return
		}
		d.showPatch(c, &d.historyPatch).Grow(1).FillHeight()
	})
}

func (d *Desktop) showHistory(c *native.Context, t *native.Theme) {
	native.Row(c).Gap(controlSpacing).Children(func() {
		currentOnly := d.historyCurrentOnly
		native.Checkbox(c, &d.historyCurrentOnly, d.copy("仅当前分支", "Current branch only")).Disabled(d.busy)
		if currentOnly != d.historyCurrentOnly {
			d.refresh()
		}
		native.TextInput(c, &d.query).Placeholder(d.text("history.search")).Grow(1).Disabled(d.busy)
		if IconButton(c, d.text("search"), "search", false).Disabled(d.busy).Clicked() {
			d.refresh()
		}
	})
	d.history.Selected = &d.logRow
	d.history.NearEnd = 12
	d.history.EndReached = func(_ int) {
		if d.historyLoadError == "" {
			d.loadMoreHistory()
		}
	}
	d.history.HeaderCell = func(cell native.Element, col int) {
		if col == 0 {
			cell.Tooltip(d.copy("圆点表示普通提交，菱形表示合并提交。", "Circles are commits; diamonds are merge commits."))
		}
	}
	showList := func() {
		native.Themed(c, t).Fill().MinHeight(0).Background(t.Surface).Children(func() {
			native.Column(c).Fill().MinHeight(0).Gap(controlSpacing).Background(t.Surface).Clip().Children(func() {
				previous := d.logRow
				native.Column(c).Grow(1).MinHeight(110).FillWidth().Padding(panelSpacing).Gap(controlSpacing).
					Background(t.Background).Radius(8).Children(func() {
					table := roundedTable(c, &d.history, []native.TableColumn{
						{Title: d.copy("分支图", "Graph"), Width: max(56, d.historyGraphWidth), Fixed: true},
						{Title: d.text("nav.commit"), Width: 110}, {Title: d.text("subject")}, {Title: d.text("author"), Width: 130}, {Title: d.text("date"), Width: 190},
					}, len(d.logs), func(row, col int) {
						l := d.logs[row]
						if col == 0 {
							if row < len(d.historyGraph) {
								graph := d.historyGraph[row]
								native.Box(c).Absolute().Left(8).Right(0).Top(0).Bottom(0).Draw(graph.draw)
							}
							return
						}
						if col == 2 && l.Refs != "" {
							native.Row(c).FillWidth().MinWidth(0).Gap(6).AlignItems(native.Center).Tooltip(l.Refs + " · " + l.Subject).Children(func() {
								native.Text(c, l.Refs).MaxWidth(140).SingleLine().Ellipsis("…").Bold()
								native.Text(c, l.Subject).Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
							})
							return
						}
						values := []string{l.Hash[:min(10, len(l.Hash))], l.Subject, l.Author, l.Date}
						value := values[col-1]
						if col == 4 {
							value = displayDateTime(value)
							native.Text(c, value).SingleLine().Ellipsis("…").Tooltip(value)
						} else {
							native.Text(c, value)
						}
					}).Disabled(d.busy)
					noCommits := len(d.logs) == 0 && !d.busy && !d.historyLoading && d.historyLoadError == "" && d.err == ""
					tableOrEmptyState(c, table, noCommits, emptyPreviewArt, d.text("history.empty"))
					if !noCommits {
						table.MinHeight(110)
					}
				})
				if previous != d.logRow && d.logRow >= 0 && d.logRow < len(d.logs) && !d.busy && d.Client != nil {
					id, commit := d.selected, d.logs[d.logRow]
					d.task(func(ctx context.Context) (func(), error) {
						var detail app.CommitDetails
						e := d.Client.Call(ctx, "git.commit.details", ipc.Params{RepoID: id, Hash: commit.Hash}, &detail)
						return func() {
							if d.selected != id || d.logRow < 0 || d.logRow >= len(d.logs) || d.logs[d.logRow].Hash != commit.Hash {
								return
							}
							d.historyShown = commit
							d.historySummary = &detail.Summary
							d.setHistoryPatch(detail.Patch)
						}, e
					})
				}
				native.Row(c).Gap(controlSpacing).Children(func() {
					valid := d.logRow >= 0 && d.logRow < len(d.logs)
					if IconButton(c, d.text("recover.undo"), "undo", false).Disabled(d.busy || !valid || d.historyQuery != "" || !d.isHistoryHead()).Clicked() && valid {
						d.confirmRecovery("undo", d.logs[d.logRow].Hash)
					}
					if IconButton(c, d.text("recover.revert"), "revert", false).Disabled(d.busy || !valid).Clicked() && valid {
						d.confirmRecovery("revert", d.logs[d.logRow].Hash)
					}
					if IconButton(c, d.text("cherry.start"), "branch", false).Disabled(d.busy || !valid || d.snapshot.CherryPicking).Clicked() && valid {
						commit := d.logs[d.logRow]
						question, mainline := "cherry.confirm", 0
						if len(commit.Parents) > 1 {
							question, mainline = "cherry.confirmMerge", 1
						}
						d.askAction(d.text(question), commit.Hash, func() { d.runCherry("start", commit.Hash, mainline) })
					}
				})
			})
		})
	}
	if d.historySummary != nil && d.logRow >= 0 && d.logRow < len(d.logs) {
		if d.historySplit == 0 {
			d.historySplit = 50
		}
		dividerTheme := *t
		dividerTheme.Border = t.Surface
		dividerTheme.Accent = t.Border
		native.Themed(c, &dividerTheme).Grow(1).MinHeight(0).FillWidth().Background(t.Surface).Children(func() {
			split := native.SplitVerticalPercent(c, &d.historySplit, func() {
				native.Column(c).Fill().MinHeight(0).Background(t.Surface).Padding(0, 0, (panelSpacing-1)/2, 0).Children(showList)
			}, func() {
				native.Themed(c, t).Fill().MinHeight(0).Background(t.Surface).Padding(panelSpacing/2, 0, 0, 0).Children(func() {
					native.Column(c).Fill().MinHeight(0).Padding(panelSpacing).Gap(controlSpacing).Background(t.Background).Radius(8).Clip().Children(func() {
						commit := d.historyShown
						if commit.Hash == "" {
							commit = d.logs[d.logRow]
						}
						native.Row(c).Gap(controlSpacing).AlignItems(native.Center).Children(func() {
							native.Segmented(c, &d.historyMode, d.text("commit.details"), d.copy("提交差异", "Commit diff"))
							native.Text(c, commit.Hash).Grow(1).MinWidth(0).SingleLine().Ellipsis("…").Tooltip(commit.Hash).TextColor(t.TextMuted)
							label := d.copy("关闭提交详情", "Close commit details")
							if IconButton(c, "", "close", false).Label(label).Tooltip(label).Clicked() {
								d.logRow, d.historySummary, d.historyShown = -1, nil, app.Commit{}
								d.setHistoryPatch("")
							}
						})
						if d.historySummary != nil {
							if d.historyMode == 0 {
								d.showCommitSummary(c, commit)
							} else {
								d.showHistoryPatch(c)
							}
						}
					})
				})
			}).Fill().SplitReset(50)
			if split.Changed() {
				d.historySplit = max(25, min(75, d.historySplit))
			}
		})
	} else {
		native.Column(c).Grow(1).MinHeight(0).FillWidth().Children(showList)
	}
	if d.historyLoadError != "" {
		native.Row(c).Gap(controlSpacing).Children(func() {
			native.Text(c, d.historyLoadError).Grow(1).TextColor(t.Danger)
			if IconButton(c, d.text("refresh"), "refresh", false).Disabled(d.busy).Clicked() {
				if d.historyMore {
					d.loadMoreHistory()
				} else {
					d.refresh()
				}
			}
		})
	} else if d.historyLoading {
		native.Text(c, d.text("busy")).TextColor(t.TextMuted)
	}
}
