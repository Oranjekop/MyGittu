package ui

import (
	"context"
	"path"
	"strings"

	native "github.com/egoist/mygo/ui"
	"mygittu/internal/app"
	"mygittu/internal/ipc"
)

const projectFilesPage = 7

type projectEntry struct {
	path string
	dir  bool
}

func (d *Desktop) rebuildProjectTree() {
	d.projectTree = d.projectTree[:0]
	query := strings.ToLower(strings.TrimSpace(d.projectSearch))
	seen := make(map[string]bool)
	for _, file := range d.projectFiles {
		if !strings.Contains(strings.ToLower(file), query) {
			continue
		}
		parts := strings.Split(file, "/")
		var dir string
		hidden := false
		for _, part := range parts[:len(parts)-1] {
			dir = path.Join(dir, part)
			if !seen[dir] {
				d.projectTree = append(d.projectTree, projectEntry{dir, true})
				seen[dir] = true
			}
			if query == "" && d.projectCollapsed[dir] {
				hidden = true
				break
			}
		}
		if !hidden {
			d.projectTree = append(d.projectTree, projectEntry{path: file})
		}
	}
}

func (d *Desktop) showProjectFiles(c *native.Context, t *native.Theme) {
	native.Row(c).Grow(1).FillWidth().Gap(panelSpacing).Children(func() {
		native.Column(c).Width(342).FillHeight().Padding(panelSpacing).Gap(controlSpacing).Background(t.Background).Radius(8).Children(func() {
			before := d.projectSearch
			native.TextInput(c, &d.projectSearch).Placeholder(d.text("files.search")).FillWidth().Disabled(d.busy)
			if before != d.projectSearch {
				d.projectFileRow, d.projectPreview = -1, app.Comparison{}
				d.rebuildProjectTree()
			}
			if len(d.projectFiles) == 0 && !d.busy {
				native.Text(c, d.text("files.empty")).TextColor(t.TextMuted)
			}
			d.projectFileList.Selected = &d.projectFileRow
			previous := d.projectFileRow
			native.Table(c, &d.projectFileList, []native.TableColumn{{Title: d.text("nav.files")}}, len(d.projectTree), func(row, _ int) {
				entry := d.projectTree[row]
				label := path.Base(entry.path)
				if entry.dir {
					prefix := "▾ "
					if d.projectSearch == "" && d.projectCollapsed[entry.path] {
						prefix = "▸ "
					}
					label = prefix + label
				}
				native.Text(c, label).Padding(0, 0, 0, float32(strings.Count(entry.path, "/")*12)).Tooltip(entry.path)
			}).Grow(1).FillWidth().Disabled(d.busy)
			if row := d.projectFileRow; row != previous && row >= 0 && row < len(d.projectTree) && !d.busy {
				entry := d.projectTree[row]
				if entry.dir {
					if d.projectCollapsed == nil {
						d.projectCollapsed = make(map[string]bool)
					}
					d.projectCollapsed[entry.path] = !d.projectCollapsed[entry.path]
					d.projectFileRow, d.projectPreview = -1, app.Comparison{}
					d.rebuildProjectTree()
					return
				}
				id, path := d.selected, entry.path
				d.projectPreview = app.Comparison{}
				d.task(func(ctx context.Context) (func(), error) {
					var preview app.Comparison
					err := d.Client.Call(ctx, "git.file.preview", ipc.Params{RepoID: id, Path: path}, &preview)
					return func() { d.projectPreview = preview }, err
				})
			}
		})
		native.Column(c).Grow(1).FillHeight().Padding(panelSpacing).Gap(controlSpacing).Background(t.Background).Radius(8).Children(func() {
			if row := d.projectFileRow; row < 0 || row >= len(d.projectTree) {
				native.Text(c, d.text("files.select")).TextColor(t.TextMuted)
			} else {
				native.Text(c, d.projectTree[row].path).Bold()
				if native.Button(c, d.text("file.history")).Disabled(d.busy).Clicked() {
					d.openFileHistory(d.projectTree[row].path)
				}
				if d.projectPreview.Binary {
					native.Text(c, d.text("diff.binary")).TextColor(t.TextMuted)
				} else if !d.busy && d.err == "" {
					native.TextArea(c, &d.projectPreview.New).Grow(1).FillWidth().ReadOnly(true)
				}
			}
		})
	})
}
