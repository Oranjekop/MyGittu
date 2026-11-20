package ui

import (
	"path"
	"slices"

	native "github.com/egoist/mygo/ui"
	git "mygittu/internal/git"
)

type changeTab struct {
	path   string
	staged bool
}

func (d *Desktop) rememberChangeTab(path string) {
	if i := slices.IndexFunc(d.changeTabs, func(tab changeTab) bool { return tab.path == path }); i >= 0 {
		d.changeTabs[i].staged = d.staged
	} else {
		d.changeTabs = append(d.changeTabs, changeTab{path: path, staged: d.staged})
	}
}

func (d *Desktop) activateChangeTab(tab changeTab) {
	row := slices.IndexFunc(d.changes, func(file git.FileStatus) bool { return file.Path == tab.path })
	if row < 0 {
		return
	}
	d.row, d.staged = row, workspaceDiffSide(d.changes[row], tab.staged)
	if d.staged {
		d.stagedCollapsed = false
	} else {
		d.changesCollapsed = false
	}
	d.diff(tab.path)
}

func (d *Desktop) showChangeTabs(c *native.Context) {
	tabs := d.changeTabs
	if len(tabs) == 0 {
		return
	}
	active := -1
	if d.row >= 0 && d.row < len(d.changes) {
		active = slices.IndexFunc(tabs, func(tab changeTab) bool { return tab.path == d.changes[d.row].Path })
	}
	selected := max(0, active)
	closing := -1
	activated := false
	d.fileTabBar(c, &d.changeTabBar, func() {
		parts := native.TabsBase(c, &selected, len(tabs))
		parts.List.Gap(detailSpacing).Children(func() {
			for i, tab := range tabs {
				name := path.Base(tab.path)
				item := parts.Tab(i).Label(name).Tooltip(tab.path).Height(controlHeight(c)).
					Padding(0, 4, 0, 8).Gap(6).AlignItems(native.Center).Radius(c.Theme().Radius).Disabled(d.busy)
				if i == active {
					item.Background(c.Theme().SurfacePressed)
				} else if item.Hovered() {
					item.Background(c.Theme().SurfaceHover)
				}
				item.Children(func() {
					icon := projectFileIcon(name)
					native.Icon(c, icons[icon]).Size(14, 14).TextColor(fileIconColor(icon, c.Theme().Dark))
					native.Text(c, name).MaxWidth(180).SingleLine().Ellipsis("…")
					label := d.copy("关闭 ", "Close ") + tab.path
					close := native.ButtonBase(c).Label(label).Tooltip(label).Size(22, 22).Center().Radius(c.Theme().Radius).Disabled(d.busy)
					if close.Hovered() {
						close.Background(c.Theme().SurfaceHover)
					}
					close.Children(func() { native.Icon(c, icons["close"]).Size(12, 12) })
					if close.Clicked() {
						closing = i
					}
				})
				if active < 0 && item.Clicked() {
					selected = i
					activated = true
				}
			}
		})
		activated = activated || parts.List.Changed()
	})
	if d.busy {
		return
	}
	if closing >= 0 {
		d.changeTabs = slices.Delete(tabs, closing, closing+1)
		if closing == active {
			d.row, d.detail, d.comparison, d.binary = -1, "", nil, false
			if len(d.changeTabs) > 0 {
				d.activateChangeTab(d.changeTabs[min(closing, len(d.changeTabs)-1)])
			}
		}
	} else if activated && selected != active && selected >= 0 && selected < len(tabs) {
		d.activateChangeTab(tabs[selected])
	}
}
