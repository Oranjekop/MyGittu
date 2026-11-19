package ui

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"mygittu/internal/platform"
)

func filePanePath(window string) string {
	dir, err := platform.ConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "windows", window+"-file-pane.json")
}

func (d *Desktop) restoreFilePane(path string) {
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var width float32
	if json.Unmarshal(data, &width) == nil && width >= 284 {
		d.filePane = width
	}
}

func (d *Desktop) saveFilePane(path string) {
	if path == "" || !d.filePaneChanged {
		return
	}
	data, err := json.Marshal(d.filePane)
	if err == nil {
		err = writeWindowPreference(path, data)
	}
	if err != nil {
		log.Printf("save file pane width: %v", err)
	}
}
