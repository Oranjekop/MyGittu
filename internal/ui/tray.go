package ui

import (
	"fmt"
	"runtime"

	"github.com/egoist/mygo"
	"mygittu/internal/brand"
)

func (d *Desktop) ensureTray() bool {
	if d.tray != nil {
		return true
	}
	options := mygo.TrayOptions{Icon: brand.SmallPNG, ToolTip: "MyGittu"}
	if runtime.GOOS == "darwin" {
		options.Icon = brand.MenuBarPNG
		options.IconIsTemplate = true
	}
	tray, err := mygo.NewTray(options)
	if err != nil {
		label := d.copy("无法创建系统托盘图标：%v", "Unable to create the system tray icon: %v")
		if runtime.GOOS == "darwin" {
			label = d.copy("无法创建菜单栏图标：%v", "Unable to create the menu bar icon: %v")
		}
		d.err = fmt.Sprintf(label, err)
		return false
	}
	d.tray = tray
	show := func() { d.Activate("") }
	tray.OnClick(show)
	tray.OnRightClick(func() {
		menu := mygo.NewMenu([]*mygo.MenuItem{
			{Label: d.copy("退出 MyGittu", "Quit MyGittu"), Click: func(*mygo.MenuItem, *mygo.Window) {
				if d.hasUnsavedProjectFiles() {
					show()
				}
				mygo.App.Quit()
			}},
		})
		tray.SetMenu(menu)
		defer tray.SetMenu(nil)
		tray.PopUpMenu()
	})
	return true
}

// Activate restores the main window and optionally opens a repository from a subsequent launch.
func (d *Desktop) Activate(path string) {
	if d.win != nil && !d.win.IsDestroyed() {
		if runtime.GOOS == "darwin" {
			mygo.App.Dock.Show()
		}
		d.win.Show()
		if d.win.IsMinimized() {
			d.win.Restore()
		}
		d.win.Focus()
	}
	if path != "" {
		d.pendingLaunchPath = path
		d.openPendingLaunch()
	}
}

func (d *Desktop) openPendingLaunch() {
	if d.win == nil || d.busy || d.pendingLaunchPath == "" {
		return
	}
	path := d.pendingLaunchPath
	d.pendingLaunchPath = ""
	d.open(path)
}

func (d *Desktop) removeTray() {
	if d.tray != nil {
		tray := d.tray
		d.tray = nil
		tray.Destroy()
	}
}
