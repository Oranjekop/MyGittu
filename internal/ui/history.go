package ui

import (
	"context"
	"fmt"
	"strings"

	native "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"mygittu/internal/app"
	"mygittu/internal/ipc"
)

func (d *Desktop) setHistoryPatch(patch string) {
	files, _ := editor.ParsePatch(patch)
	d.historyFiles, d.historyFileRow = files, 0
	d.historyFileList = native.ListState{}
	d.historyPatch = patchView{lines: d.patchRows(files, true)}
	d.historyFilePatch = patchView{}
}

func (d *Desktop) showCommitSummary(c *native.Context, commit app.Commit) {
	t := c.Theme()
	native.Scroll(c).Grow(1).MinHeight(100).FillWidth().Background(t.Background).Radius(8).Children(func() {
		native.Column(c).FillWidth().Padding(panelSpacing).Gap(controlSpacing).Children(func() {
			native.Text(c, commit.Subject).Bold().FillWidth()
			native.Text(c, commit.Author+" · "+displayDateTime(commit.Date)).TextColor(t.TextMuted)
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
		native.Text(c, d.copy("此提交没有文件差异。", "This commit has no file changes.")).TextColor(c.Theme().TextMuted)
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
			name := patchFileName(d.historyFiles[row-1])
			native.Text(c, name).SingleLine().Tooltip(name)
		}).Width(225).FillHeight()
		view := &d.historyPatch
		if row := d.historyFileRow; row > 0 && row <= len(d.historyFiles) {
			if before != row || len(d.historyFilePatch.lines) == 0 {
				d.historyFilePatch = patchView{lines: d.patchRows(d.historyFiles[row-1:row], false)}
			}
			view = &d.historyFilePatch
		}
		d.showPatch(c, view).Grow(1).FillHeight()
	})
}

func (d *Desktop) showHistory(c *native.Context, t *native.Theme) {
	native.Row(c).Gap(controlSpacing).Children(func() {
		native.TextInput(c, &d.query).Placeholder(d.text("history.search")).Grow(1).Disabled(d.busy)
		if IconButton(c, d.text("search"), "search", false).Disabled(d.busy).Clicked() {
			d.offset = 0
			d.refresh()
		}
	})
	d.history.Selected = &d.logRow
	previous := d.logRow
	roundedTable(c, &d.history, []native.TableColumn{{Title: d.text("nav.commit"), Width: 110}, {Title: d.text("subject")}, {Title: d.text("author"), Width: 130}, {Title: d.text("date"), Width: 190}}, len(d.logs), func(row, col int) {
		l := d.logs[row]
		values := []string{l.Hash[:min(10, len(l.Hash))], l.Subject, l.Author, l.Date}
		value := values[col]
		if col == 3 {
			value = displayDateTime(value)
			native.Text(c, value).SingleLine().Ellipsis("…").Tooltip(value)
		} else {
			native.Text(c, value)
		}
	}).Grow(1).MinHeight(110).FillWidth().Disabled(d.busy)
	if previous != d.logRow && d.logRow >= 0 && d.logRow < len(d.logs) && !d.busy && d.Client != nil {
		id, commit := d.selected, d.logs[d.logRow]
		d.task(func(ctx context.Context) (func(), error) {
			var detail app.CommitDetails
			e := d.Client.Call(ctx, "git.commit.details", ipc.Params{RepoID: id, Hash: commit.Hash}, &detail)
			return func() {
				d.historyShown = commit
				d.historySummary = &detail.Summary
				d.setHistoryPatch(detail.Patch)
			}, e
		})
	}
	native.Row(c).Gap(controlSpacing).Children(func() {
		valid := d.logRow >= 0 && d.logRow < len(d.logs)
		if IconButton(c, d.text("recover.undo"), "undo", false).Disabled(d.busy || !valid || d.offset != 0 || d.query != "" || d.logRow != 0).Clicked() && valid {
			d.confirmRecovery("undo", d.logs[d.logRow].Hash)
		}
		if IconButton(c, d.text("recover.revert"), "revert", false).Disabled(d.busy || !valid).Clicked() && valid {
			d.confirmRecovery("revert", d.logs[d.logRow].Hash)
		}
		if IconButton(c, d.copy("拣选提交", "Cherry-pick"), "branch", false).Disabled(d.busy || !valid).Clicked() && valid {
			hash := d.logs[d.logRow].Hash
			d.askAction(d.copy("将该提交应用到当前分支？", "Apply this commit to the current branch?"), hash, func() { d.runCherry("start", hash) })
		}
	})
	if d.historySummary != nil && d.logRow >= 0 && d.logRow < len(d.logs) {
		commit := d.historyShown
		if commit.Hash == "" {
			commit = d.logs[d.logRow]
		}
		native.Row(c).Gap(controlSpacing).AlignItems(native.Center).Children(func() {
			native.Segmented(c, &d.historyMode, d.text("commit.details"), d.copy("提交差异", "Commit diff"))
			native.Text(c, commit.Hash).TextColor(t.TextMuted)
		})
		if d.historyMode == 0 {
			d.showCommitSummary(c, commit)
		} else {
			d.showHistoryPatch(c)
		}
	}
	native.Row(c).Gap(controlSpacing).Children(func() {
		if IconButton(c, d.text("previous"), "left", false).Disabled(d.busy || d.offset == 0).Clicked() {
			d.offset = max(0, d.offset-100)
			d.refresh()
		}
		if len(d.logs) == 0 {
			native.Text(c, d.text("history.empty"))
		} else {
			native.Text(c, fmt.Sprintf("%d–%d", d.offset+1, d.offset+len(d.logs)))
		}
		if IconButton(c, d.text("next"), "right", false).Disabled(d.busy || len(d.logs) < 100).Clicked() {
			d.offset += 100
			d.refresh()
		}
	})
}
