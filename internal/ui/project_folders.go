package ui

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

func (d *Desktop) projectFoldersPath(id string) string {
	if d.draftDir == "" || !validRepositoryStateID(id) {
		return ""
	}
	return filepath.Join(filepath.Dir(d.draftDir), "file-trees", id+".json")
}

func (d *Desktop) loadProjectFolders(id string) map[string]bool {
	path := d.projectFoldersPath(id)
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	var expanded map[string]bool
	if err == nil {
		err = json.Unmarshal(data, &expanded)
	}
	if err != nil {
		log.Printf("read file tree state: %v", err)
		return nil
	}
	return expanded
}

func (d *Desktop) saveProjectFolders() {
	path := d.projectFoldersPath(d.selected)
	if path == "" {
		return
	}
	data, err := json.Marshal(d.projectExpanded)
	if err == nil {
		err = writeWindowPreference(path, data)
	}
	if err != nil {
		log.Printf("save file tree state: %v", err)
	}
}
