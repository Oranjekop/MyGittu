package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	native "github.com/egoist/mygo/ui"
	git "mygittu/internal/git"
	"mygittu/internal/ipc"
)

type workspaceChangeRow struct {
	file, count int
	staged      bool
}

func workspaceChangeRows(files []git.FileStatus, stagedCollapsed, changesCollapsed bool) []workspaceChangeRow {
	rows := make([]workspaceChangeRow, 0, len(files)+2)
	for _, staged := range []bool{true, false} {
		collapsed := changesCollapsed
		if staged {
			collapsed = stagedCollapsed
		}
		header := len(rows)
		rows = append(rows, workspaceChangeRow{file: -1, staged: staged})
		for i, file := range files {
			if !workspaceChangeInGroup(file, staged) {
				continue
			}
			rows[header].count++
			if !collapsed {
				rows = append(rows, workspaceChangeRow{file: i, staged: staged})
			}
		}
	}
	return rows
}

func workspaceChangeInGroup(file git.FileStatus, staged bool) bool {
	if file.State == git.Conflicted {
		return !staged
	}
	state := file.Worktree
	if staged {
		state = file.Index
	}
	return state != "" && state != git.Clean
}

func workspaceDiffSide(file git.FileStatus, staged bool) bool {
	if workspaceChangeInGroup(file, staged) {
		return staged
	}
	return workspaceChangeInGroup(file, true)
}

func (d *Desktop) workspaceChangeList(c *native.Context, files []git.FileStatus) {
	if len(files) == 0 && !d.busy && d.err == "" {
		emptyState(c, emptyDiffArt, 88, d.copy("暂无变更", "No changes")).Grow(1)
		return
	}
	rows := workspaceChangeRows(files, d.stagedCollapsed, d.changesCollapsed)
	selected := -1
	for i, row := range rows {
		if row.file >= 0 && row.file == d.row && row.staged == d.staged {
			selected = i
			break
		}
	}
	before := selected
	d.files.Selected = &selected
	d.files.RoundRows = true
	native.List(c, &d.files, len(rows), func(index int) {
		row := rows[index]
		if row.file < 0 {
			d.workspaceChangeHeader(c, row)
			return
		}
		d.workspaceChangeItem(c, files[row.file], row.staged)
	}).FillWidth().Grow(1).Disabled(d.busy)
	if !d.busy && selected != before && selected >= 0 && selected < len(rows) {
		row := rows[selected]
		if row.file >= 0 {
			d.row, d.staged = row.file, row.staged
			d.diff(files[row.file].Path)
		}
	}
}

func (d *Desktop) workspaceChangeHeader(c *native.Context, row workspaceChangeRow) {
	collapsed, key := &d.changesCollapsed, "changes.unstaged"
	if row.staged {
		collapsed, key = &d.stagedCollapsed, "changes.staged"
	}
	arrow := "down"
	if *collapsed {
		arrow = "right"
	}
	label := fmt.Sprintf(d.text(key), row.count)
	if IconButton(c, label, arrow, false).FillWidth().Justify(native.Start).Clicked() {
		*collapsed = !*collapsed
	}
}

func (d *Desktop) workspaceChangeItem(c *native.Context, file git.FileStatus, staged bool) {
	native.Row(c).FillWidth().MinWidth(0).Height(controlHeight(c)+4).AlignItems(native.Center).
		Gap(detailSpacing).Padding(0, detailSpacing).Children(func() {
		checked := d.tools.batch[file.Path]
		native.Checkbox(c, &checked, "").Label(d.copy("批量选择 ", "Select for batch ") + file.Path)
		if checked {
			if d.tools.batch == nil {
				d.tools.batch = make(map[string]bool)
			}
			d.tools.batch[file.Path] = true
		} else {
			delete(d.tools.batch, file.Path)
		}
		dir, name := filepath.Split(file.Path)
		native.Icon(c, icons[projectFileIcon(name)]).Size(14, 14)
		native.Row(c).Grow(1).MinWidth(0).Gap(detailSpacing).AlignItems(native.Center).Tooltip(file.Path).Children(func() {
			native.Text(c, name).MinWidth(0).SingleLine().Ellipsis("…")
			if dir != "" {
				native.Text(c, strings.TrimRight(dir, "/")).Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
			}
		})
		state, method, icon, label := file.Worktree, "git.stage", "stage", d.text("stage")
		if staged {
			state, method, icon, label = file.Index, "git.unstage", "unstage", d.text("unstage")
		}
		native.Row(c).Shrink(0).Gap(controlSpacing).AlignItems(native.Center).Children(func() {
			native.Text(c, d.text("state."+string(state))).SingleLine()
			const actionWidth = 26
			if file.State == git.Conflicted {
				native.Box(c).Width(actionWidth)
				return
			}
			if IconButton(c, "", icon, false).Size(actionWidth, 22).Shrink(0).Padding(0).
				Label(label + " " + name).Tooltip(label).Disabled(d.busy).Clicked() {
				d.mutate(method, ipc.Params{Paths: stagingPaths(nil, file, !staged)})
			}
		})
	})
}
