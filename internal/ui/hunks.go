package ui

import (
	"context"

	native "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"mygittu/internal/app"
	"mygittu/internal/ipc"
)

func (d *Desktop) loadHunks(path string) {
	id := d.selected
	d.task(func(ctx context.Context) (func(), error) {
		var hunks []app.StageHunk
		err := d.Client.Call(ctx, "git.hunks", ipc.Params{RepoID: id, Path: path}, &hunks)
		return func() {
			d.hunkPath, d.hunks, d.hunksOpen = path, hunks, true
			d.hunkRow, d.hunkPreviewRow, d.hunkConfirm = 0, -1, false
			d.hunkList = native.ListState{}
		}, err
	})
}

func (d *Desktop) applyHunk(method string) {
	row := d.hunkRow
	params := ipc.Params{RepoID: d.selected, Path: d.hunkPath, Hash: d.hunks[row].Token, Offset: row}
	d.hunkConfirm = false
	d.task(func(ctx context.Context) (func(), error) {
		actionErr := d.Client.Call(ctx, method, params, nil)
		var remaining []app.StageHunk
		err := d.Client.Call(ctx, "git.hunks", ipc.Params{RepoID: params.RepoID, Path: params.Path}, &remaining)
		return func() {
			if err == nil {
				d.hunks, d.hunkPreviewRow = remaining, -1
				d.hunkRow = min(row, len(remaining)-1)
			}
			if actionErr != nil {
				d.err = actionErr.Error()
				return
			}
			d.refresh()
			if err != nil {
				d.err = err.Error()
			}
		}, nil
	})
}

func (d *Desktop) hunkDialog(c *native.Context) {
	width, height := c.Size()
	native.Modal(c, &d.hunksOpen, func() {
		native.Column(c).Width(min(820, width-72)).Gap(panelSpacing).Children(func() {
			native.Row(c).Gap(controlSpacing).AlignItems(native.Center).Children(func() {
				native.Text(c, d.hunkPath).Bold().SingleLine().Tooltip(d.hunkPath).Grow(1)
				d.toolClose(c, &d.hunksOpen)
			})
			if d.err != "" {
				native.Text(c, d.err).FillWidth().TextColor(c.Theme().Danger)
			}
			if len(d.hunks) == 0 {
				if !d.busy && d.err == "" {
					emptyState(c, emptyDiffArt, 72, d.text("hunk.empty"))
				}
				return
			}
			native.Row(c).Height(min(340, height-166)).FillWidth().Gap(panelSpacing).Children(func() {
				d.hunkList.Selected = &d.hunkRow
				roundedTable(c, &d.hunkList, []native.TableColumn{{Title: d.copy("未暂存区块", "Unstaged hunks")}}, len(d.hunks), func(row, _ int) {
					native.Textf(c, d.copy("区块 %d", "Hunk %d"), row+1)
				}).Width(170).FillHeight().Disabled(d.busy || d.hunkConfirm)
				d.hunkRow = max(0, min(d.hunkRow, len(d.hunks)-1))
				if d.hunkRow != d.hunkPreviewRow || len(d.hunkPatch.lines) == 0 {
					files, _ := editor.ParsePatch(d.hunks[d.hunkRow].Patch)
					d.hunkPatch = patchView{lines: d.patchRows(files, false)}
					d.hunkPreviewRow = d.hunkRow
				}
				d.showPatch(c, &d.hunkPatch).Grow(1).FillHeight()
			})
			native.Row(c).Gap(controlSpacing).Wrap().Children(func() {
				if d.hunkConfirm {
					native.Text(c, d.copy("丢弃此区块的未暂存改动？", "Discard the unstaged changes in this hunk?")).FillWidth()
					if native.Button(c, d.copy("确认丢弃", "Discard changes")).Clicked() {
						d.applyHunk("git.hunk.restore")
					}
					if native.Button(c, d.copy("取消", "Cancel")).Clicked() {
						d.hunkConfirm = false
					}
					return
				}
				if IconButton(c, d.copy("暂存此区块", "Stage this hunk"), "stage", true).Disabled(d.busy).Clicked() {
					d.applyHunk("git.hunk.stage")
				}
				if IconButton(c, d.copy("丢弃此区块的改动", "Discard hunk changes"), "undo", false).Disabled(d.busy).Clicked() {
					d.hunkConfirm = true
				}
			})
		})
	})
}
