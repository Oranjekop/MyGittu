package ui

import (
	"bytes"
	"image"
	"image/png"
	"sync"
	"time"

	native "github.com/egoist/mygo/ui"
	"mygittu/internal/brand"
)

const (
	RemoteFrameInterval = 140 * time.Millisecond
	RemoteFrameCount    = 5
)

var upToDateRabbitFrames = sync.OnceValue(func() [4]*native.Bitmap {
	atlas, err := png.Decode(bytes.NewReader(brand.UpToDateRabbitPNG))
	if err != nil {
		panic(err)
	}
	sheet := atlas.(interface {
		SubImage(image.Rectangle) image.Image
	})
	size := atlas.Bounds().Dx() / 2
	var frames [4]*native.Bitmap
	for i := range frames {
		x, y := (i%2)*size, (i/2)*size
		frames[i] = native.NewBitmap(sheet.SubImage(image.Rect(x, y, x+size, y+size)))
	}
	return frames
})

func UpToDateRabbitFrame(frame int) *native.Bitmap {
	return upToDateRabbitFrames()[min(frame, 3)]
}

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
		crop := rabbitFrameBounds(atlas, row, width, height)
		for col := range frames[row] {
			rect := crop.Add(image.Pt(col*width, row*height))
			frames[row][col] = native.NewBitmap(sheet.SubImage(rect))
		}
	}
	return frames
})

// Use one square crop per animation so transparent margins do not offset
// the artwork and the drawing scale stays fixed between frames.
func rabbitFrameBounds(atlas image.Image, row, width, height int) image.Rectangle {
	var bounds image.Rectangle
	for col := 0; col < 4; col++ {
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				_, _, _, alpha := atlas.At(col*width+x, row*height+y).RGBA()
				if alpha > 0 {
					bounds = bounds.Union(image.Rect(x, y, x+1, y+1))
				}
			}
		}
	}
	side := max(bounds.Dx(), bounds.Dy())
	if side == 0 || side > min(width, height) {
		return image.Rect(0, 0, width, height)
	}
	x := min(max(0, bounds.Min.X-(side-bounds.Dx())/2), width-side)
	y := min(max(0, bounds.Min.Y-(side-bounds.Dy())/2), height-side)
	return image.Rect(x, y, x+side, y+side)
}
func RemoteRabbitFrame(action string, frame int) *native.Bitmap {
	row := 0
	switch action {
	case "fetch":
		row = 1
	case "push":
		row = 2
	}
	return remoteRabbitFrames()[row][min(frame, 3)]
}
