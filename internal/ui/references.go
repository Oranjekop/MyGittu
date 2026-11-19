package ui

import (
	"cmp"
	"slices"
	"strings"
	"time"

	native "github.com/egoist/mygo/ui"
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
			}).FillWidth().Disabled(d.busy)
			tableOrEmptyState(c, table, len(d.references.Stashes) == 0 && !d.busy && d.err == "", emptyPreviewArt, d.copy("暂无储藏", "No stashes"))
			native.TextInput(c, &d.stashMessage).Placeholder(d.text("stash.message")).FillWidth().Disabled(d.busy)
			native.Row(c).Gap(controlSpacing).Children(func() {
				if IconButton(c, d.text("stash.save"), "stash", true).Disabled(d.busy).Clicked() {
					edit("stash.save", d.stashMessage)
				}
				valid := d.stashRow >= 0 && d.stashRow < len(d.references.Stashes)
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
			}).FillWidth().Disabled(d.busy)
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
