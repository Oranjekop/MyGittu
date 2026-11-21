package ui

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"

	native "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"mygittu/internal/app"
	"mygittu/internal/ipc"
)

type projectTab struct {
	path      string
	preview   app.FilePreview
	bitmap    *native.Bitmap
	code      *editor.State
	formatted bool
	loaded    bool
}

func (d *Desktop) findProjectTab(name string) *projectTab {
	for _, tab := range d.projectTabs[d.selected] {
		if tab.path == name {
			return tab
		}
	}
	return nil
}

func (d *Desktop) rememberProjectTab() {
	if tab := d.findProjectTab(d.projectShownPath); tab != nil {
		tab.preview, tab.bitmap, tab.code, tab.formatted = d.projectPreview, d.projectBitmap, d.projectCode, d.projectFormatted
		tab.loaded = true
	}
}

func (d *Desktop) openProjectTab(name string) {
	if tab := d.findProjectTab(name); tab != nil {
		if tab.loaded {
			d.activateProjectTab(tab)
			return
		}
	} else if d.Client != nil {
		if d.projectTabs == nil {
			d.projectTabs = make(map[string][]*projectTab)
		}
		d.projectTabs[d.selected] = append(d.projectTabs[d.selected], &projectTab{path: name})
	}
	d.loadProjectPreview(name)
}

func (d *Desktop) activateProjectTab(tab *projectTab) {
	d.cancelProjectPreview()
	d.projectLoadError, d.projectFailedPath = "", ""
	d.rememberProjectTab()
	d.projectShownPath = tab.path
	d.projectPreview, d.projectBitmap, d.projectCode = tab.preview, tab.bitmap, tab.code
	d.projectLargeText = tab.preview.NextOffset != 0
	d.projectCodePath, d.projectCodeSource = tab.path, tab.preview.New
	d.projectFormatted, d.projectFormatError = tab.formatted, ""
	d.projectFileRow = slices.IndexFunc(d.projectTree, func(entry projectEntry) bool { return !entry.dir && entry.path == tab.path })
}

func (tab *projectTab) dirty() bool {
	return tab.code != nil && !tab.formatted && tab.code.Buffer.Modified()
}

func (d *Desktop) projectDirty() bool {
	return d.projectCode != nil && !d.projectFormatted && d.projectCode.Buffer.Modified()
}

func (d *Desktop) hasUnsavedProjectFiles() bool {
	d.rememberProjectTab()
	for _, tabs := range d.projectTabs {
		for _, tab := range tabs {
			if tab.dirty() {
				return true
			}
		}
	}
	return false
}

func (d *Desktop) showProjectTabs(c *native.Context) {
	tabs := d.projectTabs[d.selected]
	if len(tabs) == 0 {
		return
	}
	d.rememberProjectTab()
	active := slices.IndexFunc(tabs, func(tab *projectTab) bool { return tab.path == d.projectShownPath })
	selected := max(0, active)
	closeAll := false
	d.fileTabBar(c, &d.projectTabBar, func() {
		parts := native.TabsBase(c, &selected, len(tabs))
		parts.List.Gap(detailSpacing).Children(func() {
			for i, tab := range tabs {
				name := path.Base(tab.path)
				label := name
				if tab.dirty() {
					label += "*"
				}
				closeLabel := d.copy("关闭 ", "Close ") + tab.path
				item := parts.Tab(i).Label(label).Tooltip(tab.path).Height(controlHeight(c)).
					Padding(0, 4, 0, 8).Gap(6).AlignItems(native.Center).Radius(c.Theme().Radius).Disabled(d.busy)
				d.tabContextMenu(item, func() { closeAll = true })
				switch {
				case i == active:
					item.Background(c.Theme().SurfacePressed)
				case item.Hovered():
					item.Background(c.Theme().SurfaceHover)
				}
				item.Children(func() {
					icon := projectFileIcon(name)
					native.Icon(c, icons[icon]).Size(14, 14).TextColor(fileIconColor(icon, c.Theme().Dark))
					native.Text(c, label).MaxWidth(180).SingleLine().Ellipsis("…")
					close := native.ButtonBase(c).Label(closeLabel).Tooltip(closeLabel).Size(22, 22).Center().Radius(c.Theme().Radius)
					if close.Hovered() {
						close.Background(c.Theme().SurfaceHover)
					}
					close.Children(func() { native.Icon(c, icons["close"]).Size(12, 12) })
					if close.Clicked() {
						if tab.dirty() {
							d.askAction(d.copy("放弃未保存的修改？", "Discard unsaved edits?"), tab.path, func() { d.closeProjectTab(tab) })
						} else {
							d.closeProjectTab(tab)
						}
					}
				})
				if !d.busy && item.Clicked() && active < 0 && i == 0 {
					d.openProjectTab(tab.path)
				}
			}
		})
		if parts.List.Changed() && active != selected && !d.busy {
			d.openProjectTab(tabs[selected].path)
		}
	})
	if closeAll {
		d.requestCloseAllProjectTabs()
	}
}

func (d *Desktop) requestCloseAllProjectTabs() {
	d.rememberProjectTab()
	id := d.selected
	closeAll := func() {
		delete(d.projectTabs, id)
		if d.selected == id {
			d.clearProjectTabPreview()
		}
	}
	if slices.ContainsFunc(d.projectTabs[id], (*projectTab).dirty) {
		d.askAction(d.copy("关闭所有标签并放弃未保存的修改？", "Close all tabs and discard unsaved edits?"), "", closeAll)
		return
	}
	closeAll()
}

func (d *Desktop) closeProjectTab(tab *projectTab) {
	tabs := d.projectTabs[d.selected]
	i := slices.Index(tabs, tab)
	if i < 0 {
		return
	}
	d.projectTabs[d.selected] = slices.Delete(slices.Clone(tabs), i, i+1)
	if d.projectLoad != nil && d.projectLoad.path == tab.path {
		d.cancelProjectPreview()
	}
	if tab.path != d.projectShownPath {
		return
	}
	d.clearProjectTabPreview()
	if remaining := d.projectTabs[d.selected]; len(remaining) > 0 {
		d.openProjectTab(remaining[min(i, len(remaining)-1)].path)
	}
}

func (d *Desktop) clearProjectTabPreview() {
	d.cancelProjectPreview()
	d.projectShownPath, d.projectLoadError, d.projectFailedPath = "", "", ""
	d.projectPreview, d.projectCode, d.projectBitmap = app.FilePreview{}, nil, nil
	d.projectFileRow, d.projectLargeText = -1, false
}

func (d *Desktop) projectEditable() bool {
	size, _, ok := strings.Cut(d.projectPreview.Version, ":")
	n, err := strconv.ParseInt(size, 10, 64)
	return ok && err == nil && n >= 0 && n <= app.MaxEditableFileBytes
}

func (d *Desktop) saveProjectTab() {
	if d.Client == nil || d.busy || !d.projectDirty() {
		return
	}
	d.cancelProjectPreview()
	code := d.projectCode
	bufferVersion := code.Buffer.Version()
	if code.Buffer.Len() > app.MaxEditableFileBytes {
		d.err = d.copy("文件超过 5 MiB，无法保存。", "The file exceeds the 5 MiB save limit.")
		return
	}
	params := ipc.Params{RepoID: d.selected, Path: d.projectShownPath, PreviewVersion: d.projectPreview.Version}
	d.task(func(ctx context.Context) (func(), error) {
		var data bytes.Buffer
		if _, err := code.Buffer.WriteTo(&data); err != nil {
			return nil, err
		}
		if data.Len() > app.MaxEditableFileBytes {
			return nil, fmt.Errorf("file exceeds the 5 MiB save limit")
		}
		params.FileContent = data.Bytes()
		var version string
		err := d.Client.Call(ctx, "git.file.save", params, &version)
		return func() {
			d.projectLoadError = ""
			d.projectPreview.Version = version
			if !d.projectLargeText {
				d.projectPreview.New = data.String()
			}
			if code.Buffer.Version() == bufferVersion {
				code.Buffer.MarkSaved()
			}
			d.rememberProjectTab()
		}, err
	})
}
