package ui

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"

	native "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"mygittu/internal/ipc"
)

const referencesPage = 8

func (d *Desktop) showReferences(c *native.Context, t *native.Theme) {
	edit := func(action, name string) { d.mutate("git.reference", ipc.Params{Action: action, Name: name}) }
	native.Row(c).Grow(1).FillWidth().Gap(panelSpacing).Children(func() {
		native.Column(c).Grow(1).FillHeight().Padding(panelSpacing).Gap(controlSpacing).Background(t.Background).Radius(8).Children(func() {
			d.stashList.Selected = &d.stashRow
			table := roundedTable(c, &d.stashList, []native.TableColumn{{Title: d.text("stash.title")}}, len(d.references.Stashes), func(row, _ int) {
				_, label, _ := strings.Cut(d.references.Stashes[row], "\t")
				native.Text(c, label)
			}).Disabled(d.busy)
			tableOrEmptyState(c, table, len(d.references.Stashes) == 0 && !d.busy && d.err == "", emptyPreviewArt, d.copy("暂无储藏", "No stashes"))
			native.TextInput(c, &d.stashMessage).Placeholder(d.text("stash.message")).FillWidth().Disabled(d.busy)
			native.Row(c).Gap(controlSpacing).Children(func() {
				if IconButton(c, d.text("stash.save"), "stash", true).Disabled(d.busy).Clicked() {
					edit("stash.save", d.stashMessage)
				}
				valid := d.stashRow >= 0 && d.stashRow < len(d.references.Stashes)
				if IconButton(c, d.copy("预览储藏", "Preview stash"), "diff", false).Disabled(d.busy || !valid).Clicked() && valid {
					hash, label, _ := strings.Cut(d.references.Stashes[d.stashRow], "\t")
					d.openStashPreview(hash, label)
				}
				if IconButton(c, d.text("stash.apply"), "restore", false).Disabled(d.busy || !valid).Clicked() && valid {
					name, _, _ := strings.Cut(d.references.Stashes[d.stashRow], "\t")
					edit("stash.apply", name)
				}
			})
		})
		native.Column(c).Grow(1).FillHeight().Padding(panelSpacing).Gap(controlSpacing).Background(t.Background).Radius(8).Children(func() {
			tags := d.sortedTagNames()
			d.tagList.Selected = &d.tagRow
			d.tagList.Sort = &d.tagSort
			table := roundedTable(c, &d.tagList, []native.TableColumn{
				{ID: "name", Title: d.text("tag.title"), Sortable: true},
				{ID: "time", Title: d.text("tag.time"), Width: 214, Sortable: true},
			}, len(tags), func(row, col int) {
				name := tags[row]
				if col == 0 {
					native.Text(c, name).MinWidth(0).SingleLine().Ellipsis("…").Tooltip(name)
					return
				}
				if stamp, ok := d.references.TagTimes[name]; ok {
					value := formatLocalTime(time.Unix(stamp.Unix, 0))
					if !stamp.Annotated {
						value += " · " + d.text("tag.commitTime")
					}
					native.Text(c, value).MinWidth(0).SingleLine().Ellipsis("…").Tooltip(value)
				} else {
					native.Text(c, "—")
				}
			}).Disabled(d.busy)
			tableOrEmptyState(c, table, len(tags) == 0 && !d.busy && d.err == "", emptyPreviewArt, d.copy("暂无标签", "No tags"))
			native.TextInput(c, &d.tagName).Placeholder(d.text("tag.name")).FillWidth().Disabled(d.busy)
			native.Row(c).Gap(controlSpacing).Children(func() {
				if IconButton(c, d.text("tag.create"), "tag", true).Disabled(d.busy || strings.TrimSpace(d.tagName) == "").Clicked() {
					edit("tag.create", strings.TrimSpace(d.tagName))
				}
				valid := d.tagRow >= 0 && d.tagRow < len(tags)
				if IconButton(c, d.text("tag.delete"), "remove", false).Disabled(d.busy || !valid).Clicked() && valid {
					edit("tag.delete", tags[d.tagRow])
				}
			})
		})
	})
}

type stashPreview struct {
	open  bool
	label string
	row   int
	list  native.ListState
	files []editor.FilePatch
	patch patchView
}

func (d *Desktop) openStashPreview(hash, label string) {
	id := d.selected
	d.task(func(ctx context.Context) (func(), error) {
		var patch string
		if err := d.Client.Call(ctx, "git.stash.patch", ipc.Params{RepoID: id, Hash: hash}, &patch); err != nil {
			return nil, err
		}
		files, err := editor.ParsePatch(patch)
		return func() {
			d.stashPreview = stashPreview{open: true, label: label, files: files, row: -1}
		}, err
	})
}

func (d *Desktop) showStashPreview(c *native.Context) {
	v := &d.stashPreview
	width, height := c.Size()
	native.Modal(c, &v.open, func() {
		native.Column(c).Width(min(880, width-72)).Gap(controlSpacing).Children(func() {
			native.Row(c).FillWidth().Gap(controlSpacing).AlignItems(native.Center).Children(func() {
				native.Text(c, v.label).Grow(1).MinWidth(0).Bold().SingleLine().Ellipsis("…").Tooltip(v.label)
				if IconButton(c, d.text("close"), "close", false).Clicked() {
					v.open = false
				}
			})
			native.Row(c).Height(min(440, height-130)).FillWidth().Gap(panelSpacing).Children(func() {
				previous := v.row
				v.list.Selected = &v.row
				roundedTable(c, &v.list, []native.TableColumn{{Title: d.copy("变更文件", "Changed files")}}, len(v.files), func(row, _ int) {
					name := patchFileName(v.files[row])
					native.Text(c, name).SingleLine().Ellipsis("…").Tooltip(name)
				}).Width(230 + panelSpacing).FillHeight()
				if v.row != previous && v.row >= 0 && v.row < len(v.files) {
					v.patch = patchView{lines: d.patchRows(v.files[v.row:v.row+1], false)}
				}
				if v.row >= 0 && v.row < len(v.files) {
					d.showPatch(c, &v.patch).Grow(1).FillHeight()
				}
			})
		})
	})
	if !v.open && v.files != nil {
		*v = stashPreview{}
	}
}

func (d *Desktop) sortedTagNames() []string {
	if d.tagSort.Column == "" {
		d.tagSort = native.SortOrder{Column: "name", Descending: true}
	}
	if d.tagOrder != nil && d.tagAppliedSort == d.tagSort {
		return d.tagOrder
	}
	selected := ""
	if d.tagRow >= 0 && d.tagRow < len(d.tagOrder) {
		selected = d.tagOrder[d.tagRow]
	}
	d.tagOrder = slices.Clone(d.references.Tags)
	if d.tagSort.Column == "time" {
		slices.SortStableFunc(d.tagOrder, func(a, b string) int {
			left, leftOK := d.references.TagTimes[a]
			right, rightOK := d.references.TagTimes[b]
			if leftOK != rightOK {
				if leftOK {
					return -1
				}
				return 1
			}
			if d.tagSort.Descending {
				return cmp.Compare(right.Unix, left.Unix)
			}
			return cmp.Compare(left.Unix, right.Unix)
		})
	} else if !d.tagSort.Descending {
		slices.Reverse(d.tagOrder)
	}
	d.tagAppliedSort = d.tagSort
	d.tagRow = slices.Index(d.tagOrder, selected)
	return d.tagOrder
}
