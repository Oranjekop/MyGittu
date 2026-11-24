package ui

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/egoist/mygo"
	"mygittu/internal/brand"
	"mygittu/internal/platform"
)

// NewRememberedWindow keeps each independently launched window's state in its
// own file, so closing one process cannot overwrite another window's placement.
func NewRememberedWindow(options mygo.WindowOptions) *mygo.Window {
	if dir, err := platform.ConfigDir(); err == nil {
		mygo.App.SetWindowStateStore(windowStateFile{
			key:  options.StateKey,
			path: filepath.Join(dir, "windows", options.StateKey+".json"),
		})
	}
	taskbarPopup := runtime.GOOS == "windows" && options.StateKey != "workspace" && !options.SkipTaskbar
	showAfterBranding := taskbarPopup && !options.Hidden
	if showAfterBranding {
		options.Hidden = true
	}
	window := mygo.NewWindow(options)
	if runtime.GOOS != "darwin" && !options.SkipTaskbar {
		_ = window.SetIcon(brand.PNG)
		_ = mygo.App.Dock.SetIcon(brand.PNG)
	}
	if taskbarPopup {
		if err := setTaskbarIdentity(window.NativeHandle()); err != nil {
			log.Printf("set MyGittu taskbar identity: %v", err)
		}
	}
	if showAfterBranding {
		window.Show()
	}
	return window
}

type windowStateFile struct {
	key  string
	path string
}

func (f windowStateFile) LoadWindowStates() (map[string]mygo.WindowState, error) {
	data, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var state mygo.WindowState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return map[string]mygo.WindowState{f.key: state}, nil
}

func (f windowStateFile) SaveWindowStates(states map[string]mygo.WindowState) error {
	state, ok := states[f.key]
	if !ok {
		if err := os.Remove(f.path); !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return writeWindowPreference(f.path, data)
}

func writeWindowPreference(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".placement-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	_, writeErr := temp.Write(data)
	closeErr := temp.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}
