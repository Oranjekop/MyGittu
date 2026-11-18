package ui

import (
	"bytes"
	"context"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"image/png"
	"math"
	"mygittu/internal/brand"
	"time"
)

var brandBitmap = func() *ui.Bitmap {
	im, e := png.Decode(bytes.NewReader(brand.SmallPNG))
	if e != nil {
		panic(e)
	}
	return ui.NewBitmap(im)
}()

func OperationIcon(c *ui.Context, size float32, frame int, action string) {
	phase := float64(frame) / 24
	hop := float32(-math.Pow(math.Sin(phase*math.Pi), 2)) * size * .13
	direction := float32(1)
	if action == "push" {
		direction = -1
	}
	ui.Row(c).Width(size).Height(size).AlignItems(ui.Center).Children(func() {
		ui.Image(c, brandBitmap).Width(size*.69).Height(size*.69).Translate(0, hop)
		if symbol := icons[action]; symbol != nil {
			ui.Icon(c, symbol).Width(size*.29).Height(size*.29).
				TextColor(c.Theme().Accent).Translate(0, float32(math.Sin(phase*2*math.Pi))*size*.10*direction)
		}
	})
}

func (d *Desktop) animateOperation(ctx context.Context, operation string) {
	interval, frames := time.Second/24, 24
	if operation == cloneOperation {
		interval, frames = RemoteFrameInterval, RemoteFrameCount
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mygo.RunOnMain(func() {
				if ctx.Err() == nil && d.win != nil && !d.win.IsDestroyed() {
					d.operationFrame = (d.operationFrame + 1) % frames
					d.win.Invalidate()
				}
			})
		}
	}
}
