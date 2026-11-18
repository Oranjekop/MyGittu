package ui

import (
	"context"
	"errors"
	"github.com/egoist/mygo"
	"mygittu/internal/app"
	"mygittu/internal/ipc"
	"time"
)

func (d *Desktop) watchUpdates(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		checkContext, cancel := context.WithTimeout(ctx, 8*time.Second)
		var update app.UpdateInfo
		err := d.Client.Call(checkContext, "update.check", ipc.Params{}, &update)
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
	if d.updateChecking {
		return
	}
	d.updateChecking = true
	d.updateError = ""
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		var update app.UpdateInfo
		err := d.Client.Call(ctx, "update.check", ipc.Params{}, &update)
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
	d.updateInstalling = true
	d.updateError = ""
	version := d.update.Latest
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	d.updateCancel = cancel
	d.updateBytes, d.updateTotal = 0, 0
	go func() {
		defer cancel()
		err := app.InstallUpdate(ctx, version, func(done, total int64) {
			mygo.RunOnMain(func() {
				d.updateBytes, d.updateTotal = done, total
				if d.win != nil && !d.win.IsDestroyed() {
					d.win.Invalidate()
				}
			})
		})
		mygo.RunOnMain(func() {
			d.updateInstalling = false
			d.updateCancel = nil
			if err != nil && !errors.Is(err, context.Canceled) {
				d.updateError = err.Error()
			}
			if d.win != nil && !d.win.IsDestroyed() {
				d.win.Invalidate()
			}
		})
	}()
}
