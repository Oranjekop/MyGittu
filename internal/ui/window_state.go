package ui

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
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
	return mygo.NewWindow(options)
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
	dir := filepath.Dir(f.path)
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
	return os.Rename(temp.Name(), f.path)
}
