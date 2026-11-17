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
