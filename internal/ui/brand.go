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

var syncBitmap = func() *ui.Bitmap {
	bitmap, err := ui.DecodeBitmap(brand.SyncPNG)
	if err != nil {
		panic(err)
	}
	return bitmap
}()

func OperationIcon(c *ui.Context, size float32, frame int) {
	offset := float32(-3 * math.Sin(float64(frame)*2*math.Pi/24))
	ui.Image(c, syncBitmap).Width(size).Height(size).Translate(0, offset)
}

func (d *Desktop) animateOperation(ctx context.Context) {
	ticker := time.NewTicker(time.Second / 24)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mygo.RunOnMain(func() {
				if ctx.Err() == nil && d.win != nil && !d.win.IsDestroyed() {
					d.operationFrame = (d.operationFrame + 1) % 24
					d.win.Invalidate()
				}
			})
		}
	}
}
