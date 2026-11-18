package main

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	native "github.com/egoist/mygo/ui"
)

func TestRemoteAnimationKeepsTextStill(t *testing.T) {
	d := &remoteWindow{action: "pull", lang: "zh-CN", status: "正在拉取远程更新…"}
	tester := native.NewTester(d.view, 420, 180)
	first := tester.Image()
	before := append([]byte(nil), first.Pix...)
	d.frame = 6
	tester.Frame()
	after := tester.Image()
	if bytes.Equal(before, after.Pix) {
		t.Fatal("animation frame did not move the synchronization icon")
	}
	for y := 0; y < after.Bounds().Dy(); y++ {
		start, end := y*after.Stride+90*4, (y+1)*after.Stride
		if !bytes.Equal(before[start:end], after.Pix[start:end]) {
			t.Fatal("animation moved content outside the icon area")
		}
	}
	if dir := os.Getenv("MYGITTU_SNAPSHOT_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		saveRemoteFrame(t, filepath.Join(dir, "sync-running.png"), after)
		d.finished, d.frame, d.status = true, 0, "已完成"
		tester.Frame()
		saveRemoteFrame(t, filepath.Join(dir, "sync-success.png"), tester.Image())
		d.err, d.status = "无法连接远程仓库，请检查网络连接。", "操作失败"
		tester.Frame()
		saveRemoteFrame(t, filepath.Join(dir, "sync-error.png"), tester.Image())
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
