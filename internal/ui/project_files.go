package ui

import (
	"context"

	native "github.com/egoist/mygo/ui"
	"mygittu/internal/app"
	"mygittu/internal/ipc"
)

const projectFilesPage = 7

func (d *Desktop) showProjectFiles(c *native.Context, t *native.Theme) {
	native.Row(c).Grow(1).FillWidth().Gap(6).Children(func() {
		native.Column(c).Width(342).FillHeight().Padding(10).Gap(8).Background(t.Background).Radius(8).Children(func() {
			if len(d.projectFiles) == 0 && !d.busy {
				native.Text(c, d.text("files.empty")).TextColor(t.TextMuted)
			}
			d.projectFileList.Selected = &d.projectFileRow
			previous := d.projectFileRow
			native.Table(c, &d.projectFileList, []native.TableColumn{{Title: d.text("nav.files")}}, len(d.projectFiles), func(row, _ int) {
				path := d.projectFiles[row]
				native.Text(c, path).Tooltip(path)
			}).Grow(1).FillWidth().Disabled(d.busy)
			if row := d.projectFileRow; row != previous && row >= 0 && row < len(d.projectFiles) && !d.busy {
				id, path := d.selected, d.projectFiles[row]
				d.projectPreview = app.Comparison{}
				d.task(func(ctx context.Context) (func(), error) {
					var preview app.Comparison
					err := d.Client.Call(ctx, "git.file.preview", ipc.Params{RepoID: id, Path: path}, &preview)
					return func() { d.projectPreview = preview }, err
				})
			}
		})
		native.Column(c).Grow(1).FillHeight().Padding(12).Gap(8).Background(t.Background).Radius(8).Children(func() {
			if row := d.projectFileRow; row < 0 || row >= len(d.projectFiles) {
				native.Text(c, d.text("files.select")).TextColor(t.TextMuted)
			} else {
				native.Text(c, d.projectFiles[row]).Bold()
				if d.projectPreview.Binary {
					native.Text(c, d.text("diff.binary")).TextColor(t.TextMuted)
				} else if !d.busy && d.err == "" {
					native.TextArea(c, &d.projectPreview.New).Grow(1).FillWidth().ReadOnly(true)
				}
			}
		})
	})
}
