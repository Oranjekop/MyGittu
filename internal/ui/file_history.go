package ui

import (
	"context"
	"strconv"

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
			w.fileDiff = editor.DiffState{}
			w.fileShown = ""
			w.fileHistoryLoaded = true
			w.blameRows, w.blameLoaded = nil, false
			w.fileList, w.blameList = native.ListState{}, native.ListState{}
		}, err
	})
}

func (d *Desktop) isStandaloneFileHistory() bool {
	return d.action == "file-log" || d.action == "file-blame" || d.action == "file-previous"
}

func (d *Desktop) loadFileHistoryList() {
	params := ipc.Params{RepoID: d.selected, Path: d.tools.filePath, Limit: 100}
	d.task(func(ctx context.Context) (func(), error) {
		var list []app.Commit
		err := d.Client.Call(ctx, "git.file.log", params, &list)
		return func() {
			d.tools.fileHistoryLoaded, d.tools.fileCommits, d.tools.fileRow = true, list, 0
		}, err
	})
}

func (d *Desktop) showFileHistory(c *native.Context) {
	if d.isStandaloneFileHistory() {
		return
	}
	width, height := c.Size()
	native.Modal(c, &d.tools.fileOpen, func() {
		native.Column(c).Width(min(880, width-72)).Gap(panelSpacing).Children(func() {
			d.fileHistoryContent(c, height, false)
		})
	})
}

func (d *Desktop) fileHistoryContent(c *native.Context, height float32, standalone bool) {
	w := &d.tools
	native.Row(c).Gap(controlSpacing).AlignItems(native.Center).Children(func() {
		native.Text(c, w.filePath).Bold().SingleLine().Grow(1).Tooltip(w.filePath)
		if !standalone {
			d.toolClose(c, &w.fileOpen)
		}
	})
	before := w.fileMode
	if d.action != "file-previous" {
		native.Segmented(c, &w.fileMode, d.text("file.history"), d.text("file.blame")).Disabled(d.busy)
	}
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
					if !w.fileHistoryLoaded {
						d.loadFileHistoryList()
					}
				}
				d.err = ""
			}
		})
	}
	if w.fileMode == 1 {
		d.showFileBlame(c, max(100, min(425, height-175)))
		return
	}
	if !w.fileHistoryLoaded && !d.busy && d.err == "" && d.Client != nil {
		d.loadFileHistoryList()
	}
	if len(w.fileCommits) == 0 {
		if !d.busy && d.err == "" {
			state := emptyState(c, emptyPreviewArt, 72, d.text("history.empty"))
			if standalone {
				state.Grow(1)
			}
		}
		return
	}
	if d.action != "file-previous" {
		w.fileList.Selected = &w.fileRow
		roundedTable(c, &w.fileList, []native.TableColumn{
			{Title: d.text("subject")}, {Title: d.text("author"), Width: 120}, {Title: d.text("date"), Width: dateColumnWidth},
		}, len(w.fileCommits), func(row, col int) {
			commit := w.fileCommits[row]
			text := []string{commit.Subject, commit.Author, commit.Date}[col]
			if col == 2 {
				text = displayDateTime(text)
			}
			native.Text(c, text).SingleLine().Ellipsis("…").Tooltip(text)
		}).Height(min(150, height*.23, 28*float32(len(w.fileCommits)+1)+2*controlSpacing)).Disabled(d.busy)
	}
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
	if w.fileDiff.Old != nil {
		native.Text(c, shown).SingleLine().Tooltip(shown).TextColor(c.Theme().TextMuted)
		width, _ := c.Size()
		d.showCodeDiff(c, &w.fileDiff, width-2*panelSpacing, false)
	} else if len(w.filePatch.lines) > 0 {
		native.Text(c, shown).SingleLine().Tooltip(shown).TextColor(c.Theme().TextMuted)
		patch := d.showPatch(c, &w.filePatch)
		if standalone {
			patch.Grow(1).MinHeight(0)
		} else {
			patch.Height(min(280, height*.43))
		}
	} else if d.busy && w.fileShown == "" {
		native.Text(c, d.text("busy")).TextColor(c.Theme().TextMuted)
	} else if d.err == "" {
		state := emptyState(c, emptyDiffArt, 72, d.copy("没有文件差异", "No file differences"))
		if standalone {
			state.Grow(1)
		}
	}
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
			w.filePatch, w.fileDiff = patchView{}, editor.DiffState{}
			if len(files) == 1 && !files[0].Binary && len(files[0].Hunks) > 0 {
				w.fileDiff.SetPatch(files[0])
			} else {
				w.filePatch.lines = d.patchRows(files, false)
			}
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
			emptyState(c, emptyPreviewArt, 72, d.copy("没有可追溯的行。", "No lines to annotate."))
		}
		return
	}
	roundedTable(c, &w.blameList, []native.TableColumn{
		{Title: d.copy("行", "Line"), Width: 55},
		{Title: d.text("author"), Width: 130}, {Title: d.text("date"), Width: dateColumnWidth},
		{Title: d.copy("代码", "Code")},
	}, len(w.blameRows), func(row, col int) {
		line := w.blameRows[row]
		if col == 3 {
			native.Text(c, line.Text).Font("monospace").SingleLine().Tooltip(line.Text)
		} else if col == 2 {
			stamp := displayDateTime(line.Date)
			native.Text(c, stamp).SingleLine().Ellipsis("…").Tooltip(stamp)
		} else {
			native.Text(c, []string{strconv.Itoa(line.Number), line.Author}[col]).SingleLine().Tooltip(line.Hash)
		}
	}).Height(height)
}
