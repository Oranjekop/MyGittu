package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func (d *Desktop) remoteAction(action string) {
	for _, repo := range d.repos {
		if repo.ID != d.selected {
			continue
		}
		self, err := os.Executable()
		if err != nil {
			d.err = err.Error()
			return
		}
		name := filepath.Base(self)
		if runtime.GOOS == "windows" && filepath.Ext(name) == "" {
			name += ".exe"
		}
		cmd := exec.Command(filepath.Join(filepath.Dir(self), name), "--path", repo.Root, "--action", action)
		if err := cmd.Start(); err != nil {
			d.err = fmt.Sprintf("%s: %v", d.text("error"), err)
		}
		return
	}
}
