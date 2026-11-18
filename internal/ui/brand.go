package ui

import (
	"bytes"
	"github.com/egoist/mygo/ui"
	"image/png"
	"mygittu/internal/brand"
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

// RemoteBitmap returns the separate synchronization mark.
func RemoteBitmap() *ui.Bitmap { return syncBitmap }
