package ui

import (
	"context"

	native "github.com/egoist/mygo/ui"
	"mygittu/internal/app"
	"mygittu/internal/ipc"
)

func (d *Desktop) loadHunks(path string) {
	id := d.selected
	d.task(func(ctx context.Context) (func(), error) {
		var hunks []app.StageHunk
		err := d.Client.Call(ctx, "git.hunks", ipc.Params{RepoID: id, Path: path}, &hunks)
		return func() {
			d.hunkPath, d.hunks, d.hunkRow, d.hunksOpen = path, hunks, -1, true
		}, err
	})
}

func (d *Desktop) hunkDialog(c *native.Context) {
	native.Modal(c, &d.hunksOpen, func() {
		native.Column(c).Width(680).Gap(controlSpacing).Children(func() {
			native.Text(c, d.hunkPath).Bold()
			if len(d.hunks) == 0 {
				native.Text(c, d.text("hunk.empty"))
			} else {
				d.hunkList.Selected = &d.hunkRow
				native.Table(c, &d.hunkList, []native.TableColumn{{Title: d.text("hunk.stage")}}, len(d.hunks), func(row, _ int) {
					native.Text(c, d.hunks[row].Label)
				}).Height(140).FillWidth()
			}
			if d.hunkRow >= 0 && d.hunkRow < len(d.hunks) {
				hunk := d.hunks[d.hunkRow]
				native.TextArea(c, &hunk.Patch).Height(220).FillWidth().ReadOnly(true)
				if native.PrimaryButton(c, d.text("hunk.stage")).Disabled(d.busy).Clicked() {
					d.hunksOpen = false
					d.mutate("git.hunk.stage", ipc.Params{Path: d.hunkPath, Hash: hunk.Token, Offset: d.hunkRow})
				}
			}
			if native.Button(c, d.text("close")).Clicked() {
				d.hunksOpen = false
			}
		})
	})
}
