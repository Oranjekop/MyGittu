package ui

import (
	"context"
	"strconv"
	"time"

	native "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"mygittu/internal/app"
	"mygittu/internal/ipc"
)

func (d *Desktop) openFileHistory(path string) {
	id := d.selected
	d.task(func(ctx context.Context) (func(), error) {
		var list []app.Commit
		err := d.Client.Call(ctx, "git.file.log", ipc.Params{RepoID: id, Path: path, Limit: 100}, &list)
		return func() {
			w := &d.tools
			w.filePath, w.fileCommits, w.fileRow, w.fileLoaded, w.fileOpen = path, list, 0, "", true
			w.filePatch, w.fileMode = patchView{}, 0
			w.fileShown = ""
			w.blameRows, w.blameLoaded = nil, false
			w.fileList, w.blameList = native.ListState{}, native.ListState{}
		}, err
	})
}

func (d *Desktop) showFileHistory(c *native.Context) {
	w := &d.tools
	width, height := c.Size()
	native.Modal(c, &w.fileOpen, func() {
		native.Column(c).Width(min(880, width-72)).Gap(panelSpacing).Children(func() {
			native.Row(c).Gap(controlSpacing).AlignItems(native.Center).Children(func() {
				native.Text(c, w.filePath).Bold().SingleLine().Grow(1).Tooltip(w.filePath)
				d.toolClose(c, &w.fileOpen)
			})
			before := w.fileMode
			native.Segmented(c, &w.fileMode, d.text("file.history"), d.text("file.blame")).Disabled(d.busy)
			if before != w.fileMode {
				d.err = ""
			}
			if d.err != "" {
				native.Row(c).Gap(controlSpacing).Children(func() {
					native.Text(c, d.err).Grow(1).TextColor(c.Theme().Danger)
					if native.Button(c, d.copy("重试", "Retry")).Disabled(d.busy).Clicked() {
						if w.fileMode == 1 {
							w.blameLoaded = false
						} else {
							w.fileLoaded = ""
						}
						d.err = ""
					}
				})
			}
			if w.fileMode == 1 {
				d.showFileBlame(c, min(425, height-175))
				return
			}
			if len(w.fileCommits) == 0 {
				native.Text(c, d.text("history.empty")).TextColor(c.Theme().TextMuted)
				return
			}
			w.fileList.Selected = &w.fileRow
			roundedTable(c, &w.fileList, []native.TableColumn{
				{Title: d.text("subject")}, {Title: d.text("author"), Width: 120}, {Title: d.text("date"), Width: 112},
			}, len(w.fileCommits), func(row, col int) {
				commit := w.fileCommits[row]
				text := []string{commit.Subject, commit.Author, commitDate(commit.Date)}[col]
				native.Text(c, text).SingleLine().Tooltip(text)
			}).Height(min(150, height*.23, 28*float32(len(w.fileCommits)+1))).FillWidth().Disabled(d.busy)
			if w.fileRow < 0 || w.fileRow >= len(w.fileCommits) {
				return
			}
			commit := w.fileCommits[w.fileRow]
			if w.fileLoaded != commit.Hash && d.Client != nil && !d.busy {
				d.loadFilePatch(commit.Hash)
			}
			shown := w.fileShown
			if shown == "" {
				shown = commit.Hash
			}
			native.Text(c, shown).SingleLine().Tooltip(shown).TextColor(c.Theme().TextMuted)
			if len(w.filePatch.lines) > 0 {
				d.showPatch(c, &w.filePatch).Height(min(280, height*.43))
			} else if d.busy && w.fileShown == "" {
				native.Text(c, d.text("busy")).TextColor(c.Theme().TextMuted)
			} else if d.err == "" {
				native.Text(c, d.text("diff.empty")).TextColor(c.Theme().TextMuted)
			}
		})
	})
}

func commitDate(date string) string {
	if stamp, err := time.Parse(time.RFC3339, date); err == nil {
		return stamp.Format(time.DateOnly)
	}
	return date
}

func (d *Desktop) loadFilePatch(hash string) {
	w := &d.tools
	w.fileLoaded = hash
	params := ipc.Params{RepoID: d.selected, Path: w.filePath, Action: "diff", Hash: hash}
	d.task(func(ctx context.Context) (func(), error) {
		var patch string
		err := d.Client.Call(ctx, "git.file.insight", params, &patch)
		files, parseErr := editor.ParsePatch(patch)
		if err == nil {
			err = parseErr
		}
		return func() {
			w.fileShown = hash
			w.filePatch = patchView{lines: d.patchRows(files, false)}
		}, err
	})
}

func (d *Desktop) showFileBlame(c *native.Context, height float32) {
	w := &d.tools
	if !w.blameLoaded && d.Client != nil && !d.busy {
		w.blameLoaded = true
		params := ipc.Params{RepoID: d.selected, Path: w.filePath}
		d.task(func(ctx context.Context) (func(), error) {
			var lines []app.BlameLine
			err := d.Client.Call(ctx, "git.file.blame", params, &lines)
			return func() { w.blameRows = lines }, err
		})
	}
	if d.busy && len(w.blameRows) == 0 {
		native.Text(c, d.text("busy")).TextColor(c.Theme().TextMuted)
		return
	}
	if len(w.blameRows) == 0 {
		if d.err == "" {
			native.Text(c, d.copy("没有可追溯的行。", "No lines to annotate.")).TextColor(c.Theme().TextMuted)
		}
		return
	}
	roundedTable(c, &w.blameList, []native.TableColumn{
		{Title: d.copy("行", "Line"), Width: 55},
		{Title: d.text("author"), Width: 130}, {Title: d.text("date"), Width: 110},
		{Title: d.copy("代码", "Code")},
	}, len(w.blameRows), func(row, col int) {
		line := w.blameRows[row]
		if col == 3 {
			native.Text(c, line.Text).Font("monospace").SingleLine().Tooltip(line.Text)
		} else {
			native.Text(c, []string{strconv.Itoa(line.Number), line.Author, line.Date}[col]).SingleLine().Tooltip(line.Hash)
		}
	}).Height(height).FillWidth()
}
