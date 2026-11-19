package ui

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
)

func TestWindowPlacementFilesKeepIndependentBounds(t *testing.T) {
	dir := t.TempDir()
	workspace := windowStateFile{key: "workspace", path: filepath.Join(dir, "windows", "workspace.json")}
	remote := windowStateFile{key: "remote", path: filepath.Join(dir, "windows", "remote.json")}
	mainBounds := mygo.WindowState{X: -300, Y: 220, Width: 1080, Height: 740, PixelX: -450, PixelWidth: 1620}
	remoteBounds := mygo.WindowState{X: 210, Y: 180, Width: 470, Height: 260}
	if err := workspace.SaveWindowStates(map[string]mygo.WindowState{"workspace": mainBounds}); err != nil {
		t.Fatal(err)
	}
	if err := remote.SaveWindowStates(map[string]mygo.WindowState{"remote": remoteBounds}); err != nil {
		t.Fatal(err)
	}
	remoteBounds.X, remoteBounds.Width = 461, 520
	if err := remote.SaveWindowStates(map[string]mygo.WindowState{"remote": remoteBounds}); err != nil {
		t.Fatal(err)
	}
	for store, want := range map[windowStateFile]mygo.WindowState{workspace: mainBounds, remote: remoteBounds} {
		saved, err := store.LoadWindowStates()
		if err != nil || len(saved) != 1 || saved[store.key] != want {
			t.Fatalf("%s restored %+v, want %+v (error: %v)", store.key, saved, want, err)
		}
	}
}
