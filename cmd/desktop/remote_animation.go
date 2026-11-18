package main

import (
	"bytes"
	"image"
	"image/png"
	"sync"
	"time"

	native "github.com/egoist/mygo/ui"
	"mygittu/internal/brand"
)

const remoteFrameInterval = 140 * time.Millisecond

var remoteRabbitFrames = sync.OnceValue(func() [3][4]*native.Bitmap {
	atlas, err := png.Decode(bytes.NewReader(brand.RemoteRabbitPNG))
	if err != nil {
		panic(err)
	}
	sheet := atlas.(interface {
		SubImage(image.Rectangle) image.Image
	})
	width, height := atlas.Bounds().Dx()/4, atlas.Bounds().Dy()/3
	var frames [3][4]*native.Bitmap
	for row := range frames {
		for col := range frames[row] {
			rect := image.Rect(col*width, row*height, (col+1)*width, (row+1)*height)
			frames[row][col] = native.NewBitmap(sheet.SubImage(rect))
		}
	}
	return frames
})

func (d *remoteWindow) rabbitFrame() *native.Bitmap {
	row := 0
	switch d.action {
	case "fetch":
		row = 1
	case "push":
		row = 2
	}
	return remoteRabbitFrames()[row][min(d.frame, 3)]
}
