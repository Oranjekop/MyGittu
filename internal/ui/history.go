package ui

import (
	"strings"

	native "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"mygittu/internal/app"
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
			native.Text(c, commit.Author+" · "+commitDate(commit.Date)).TextColor(t.TextMuted)
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
		native.Table(c, &d.historyFileList, []native.TableColumn{{Title: d.copy("变更文件", "Changed files")}}, len(d.historyFiles)+1, func(row, _ int) {
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
