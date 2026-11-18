package main

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	native "github.com/egoist/mygo/ui"
	"mygittu/internal/brand"
)

func TestRemoteRabbitFramesAreSquare(t *testing.T) {
	atlas, err := png.DecodeConfig(bytes.NewReader(brand.RemoteRabbitPNG))
	if err != nil {
		t.Fatal(err)
	}
	if atlas.Width%4 != 0 || atlas.Height%3 != 0 || atlas.Width/4 != atlas.Height/3 {
		t.Fatalf("expected square frames in the 4×3 atlas, got %d×%d", atlas.Width, atlas.Height)
	}
}

func TestRemoteAnimationKeepsTextStill(t *testing.T) {
	for _, action := range []string{"pull", "fetch", "push"} {
		t.Run(action, func(t *testing.T) { checkRemoteAnimation(t, action) })
	}
}

func checkRemoteAnimation(t *testing.T, action string) {
	t.Helper()
	d := &remoteWindow{action: action, lang: "zh-CN"}
	tester := native.NewTester(d.view, 384, 195)
	first := tester.Image()
	before := append([]byte(nil), first.Pix...)
	d.frame = 1
	tester.Frame()
	after := tester.Image()
	if bytes.Equal(before, after.Pix) {
		t.Fatal("animation frame did not move the synchronization icon")
	}
	label, ok := tester.Find(d.runningStatus())
	if !ok || label.Y+label.H > 175 || label.X < 0 || label.X+label.W > 384 {
		t.Fatalf("status is clipped or too close to the edge: %v", label)
	}
	var artwork image.Rectangle
	background := after.RGBAAt(0, 0)
	for y := 0; y < after.Bounds().Dy(); y++ {
		for x := 0; x < after.Bounds().Dx(); x++ {
			if after.RGBAAt(x, y) != background {
				artwork = artwork.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if offset := artwork.Min.Y + artwork.Max.Y - after.Bounds().Dy(); offset < -24 || offset > 24 {
		t.Fatalf("artwork and status are not centered as a group: %v", artwork)
	}
	for y := int(label.Y); y < after.Bounds().Dy(); y++ {
		start, end := y*after.Stride, (y+1)*after.Stride
		if !bytes.Equal(before[start:end], after.Pix[start:end]) {
			t.Fatal("animation moved content outside the icon area")
		}
	}
	if dir := os.Getenv("MYGITTU_SNAPSHOT_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		saveRemoteFrame(t, filepath.Join(dir, action+"-running.png"), after)
		d.finished, d.frame, d.status = true, 3, "已完成"
		tester.Frame()
		saveRemoteFrame(t, filepath.Join(dir, action+"-success.png"), tester.Image())
		d.err, d.status = "无法连接远程仓库，请检查网络连接。", "操作失败"
		tester.Frame()
		saveRemoteFrame(t, filepath.Join(dir, action+"-error.png"), tester.Image())
	}
}

func saveRemoteFrame(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
