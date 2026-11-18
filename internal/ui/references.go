package ui

import (
	"strings"

	native "github.com/egoist/mygo/ui"
	"mygittu/internal/ipc"
)

const referencesPage = 8

func (d *Desktop) showReferences(c *native.Context, t *native.Theme) {
	edit := func(action, name string) { d.mutate("git.reference", ipc.Params{Action: action, Name: name}) }
	native.Row(c).Grow(1).FillWidth().Gap(panelSpacing).Children(func() {
		native.Column(c).Grow(1).FillHeight().Padding(panelSpacing).Gap(controlSpacing).Background(t.Background).Radius(8).Children(func() {
			d.stashList.Selected = &d.stashRow
			native.Table(c, &d.stashList, []native.TableColumn{{Title: d.text("stash.title")}}, len(d.references.Stashes), func(row, _ int) {
				_, label, _ := strings.Cut(d.references.Stashes[row], "\t")
				native.Text(c, label)
			}).Grow(1).FillWidth().Disabled(d.busy)
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
			d.tagList.Selected = &d.tagRow
			native.Table(c, &d.tagList, []native.TableColumn{{Title: d.text("tag.title")}}, len(d.references.Tags), func(row, _ int) {
				native.Text(c, d.references.Tags[row])
			}).Grow(1).FillWidth().Disabled(d.busy)
			native.TextInput(c, &d.tagName).Placeholder(d.text("tag.name")).FillWidth().Disabled(d.busy)
			native.Row(c).Gap(controlSpacing).Children(func() {
				if IconButton(c, d.text("tag.create"), "tag", true).Disabled(d.busy || strings.TrimSpace(d.tagName) == "").Clicked() {
					edit("tag.create", strings.TrimSpace(d.tagName))
				}
				valid := d.tagRow >= 0 && d.tagRow < len(d.references.Tags)
				if IconButton(c, d.text("tag.delete"), "remove", false).Disabled(d.busy || !valid).Clicked() && valid {
					edit("tag.delete", d.references.Tags[d.tagRow])
				}
			})
		})
	})
}
