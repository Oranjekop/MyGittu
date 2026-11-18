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
	tester := native.NewTester(d.view, 400, 234)
	first := tester.Image()
	before := append([]byte(nil), first.Pix...)
	d.frame = 1
	tester.Frame()
	after := tester.Image()
	if bytes.Equal(before, after.Pix) {
		t.Fatal("animation frame did not move the synchronization icon")
	}
	for y := 175; y < after.Bounds().Dy(); y++ {
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
