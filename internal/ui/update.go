package ui

import (
	"context"
	"errors"
	"fmt"
	"github.com/egoist/mygo"
	native "github.com/egoist/mygo/ui"
	"mygittu/internal/app"
	"time"
)

func (d *Desktop) watchUpdates(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		checkContext, cancel := context.WithTimeout(ctx, 8*time.Second)
		update, err := app.CheckUpdate(checkContext)
		cancel()
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			mygo.RunOnMain(func() {
				d.update = update
				if d.win != nil {
					d.win.Invalidate()
				}
			})
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d *Desktop) checkUpdate(showError bool) {
	if d.updateChecking || d.updateInstalling {
		return
	}
	d.updateChecking = true
	d.updateError = ""
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		update, err := app.CheckUpdate(ctx)
		mygo.RunOnMain(func() {
			d.updateChecking = false
			if err == nil {
				d.update = update
			} else if showError {
				d.updateError = err.Error()
			}
			if d.win != nil {
				d.win.Invalidate()
			}
		})
	}()
}

func (d *Desktop) installUpdate() {
	if d.updateInstalling || !d.update.Available {
		return
	}
	if err := d.canInstallUpdate(); err != nil {
		d.showUpdateError(err)
		return
	}
	d.updateInstalling = true
	d.updateError = ""
	version := d.update.Latest
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	d.updateCancel = cancel
	d.updateBytes, d.updateTotal = 0, 0
	go func() {
		path, err := app.DownloadUpdate(ctx, version, func(done, total int64) {
			mygo.RunOnMain(func() {
				d.updateBytes, d.updateTotal = done, total
				if d.win != nil && !d.win.IsDestroyed() {
					d.win.Invalidate()
				}
			})
		})
		mygo.RunOnMain(func() {
			defer cancel()
			d.updateInstalling = false
			d.updateCancel = nil
			if err == nil {
				err = ctx.Err()
			}
			if err == nil {
				err = d.canInstallUpdate()
			}
			if err == nil {
				err = app.OpenUpdateInstaller(path)
				if err == nil {
					mygo.App.Quit()
					return
				}
			}
			if path != "" {
				app.DiscardUpdate(path)
			}
			if !errors.Is(err, context.Canceled) {
				d.showUpdateError(err)
			}
			if d.win != nil && !d.win.IsDestroyed() {
				d.win.Invalidate()
			}
		})
	}()
}

func (d *Desktop) canInstallUpdate() error {
	if d.hasUnsavedProjectFiles() {
		return errors.New(d.copy("请先保存文件修改，再安装更新。", "Save your file changes before installing the update."))
	}
	if d.busy {
		return errors.New(d.copy("请等待当前操作结束，再安装更新。", "Wait for the current operation to finish before installing the update."))
	}
	return nil
}

func (d *Desktop) showUpdateError(err error) {
	d.settingsOpen = true
	d.settingsTab = 4
	d.updateError = err.Error()
}

func (d *Desktop) settingsToolbarButton(c *native.Context) {
	native.Box(c).Children(func() {
		label := d.text("nav.settings")
		if d.update.Available {
			label = fmt.Sprintf(d.text("update.available"), d.update.Latest)
		}
		if IconButton(c, "", "settings", false).Label(label).Tooltip(label).Disabled(d.operation == cloneOperation).Clicked() {
			d.openSettings()
			if d.update.Available {
				d.settingsTab = 4
			}
		}
		if d.update.Available {
			native.Box(c).Absolute().Top(2).Right(2).Size(7, 7).Radius(4).Background(c.Theme().Danger)
		}
	})
}
