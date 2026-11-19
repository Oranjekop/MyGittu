package ui

import (
	"bytes"
	"image/png"
	"sync"

	native "github.com/egoist/mygo/ui"
	"mygittu/internal/brand"
)

var (
	emptyDiffArt    = emptyStateBitmap(brand.EmptyDiffPNG)
	emptyPreviewArt = emptyStateBitmap(brand.EmptyPreviewPNG)
)

func emptyStateBitmap(data []byte) func() *native.Bitmap {
	return sync.OnceValue(func() *native.Bitmap {
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			panic(err)
		}
		return native.NewBitmap(img)
	})
}

func emptyState(c *native.Context, art func() *native.Bitmap, size float32, message string) native.Element {
	return native.Column(c).FillWidth().MinHeight(0).Center().Gap(controlSpacing).Children(func() {
		native.Image(c, art()).Width(size).MaxWidthPercent(100).Height(size).MinHeight(0).Shrink(1).Fit(native.Contain)
		native.Text(c, message).MaxWidthPercent(100).TextColor(c.Theme().TextMuted)
	})
}

func tableOrEmptyState(c *native.Context, table native.Element, empty bool, art func() *native.Bitmap, message string) {
	if empty {
		table.Height(28)
		emptyState(c, art, 88, message).Grow(1)
	} else {
		table.Grow(1)
	}
}
