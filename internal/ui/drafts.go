package ui

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"mygittu/internal/app"
	"mygittu/internal/status"
)

func (d *Desktop) selectRepository(id, root string) {
	if id != d.selected {
		d.saveDraft()
		d.message = d.loadDraft(id)
		d.savedMessage = d.message
		d.snapshot = status.Snapshot{}
		d.remoteNotice = ""
		d.row, d.changes, d.stagedCount = -1, nil, 0
		d.detail, d.comparison, d.binary = "", nil, false
		d.logs, d.logRow, d.historySummary, d.historyShown = nil, -1, nil, app.Commit{}
		d.historyGraph, d.historyGraphWidth = nil, 0
		d.historyGraphBuilder = commitGraphBuilder{}
		d.invalidateHistoryLoad()
		d.historyMore = false
		d.projectSearch, d.projectCollapsed = "", nil
		d.resetProjectFiles()
		d.hunksOpen = false
		d.aiError = ""
		d.tools = workbench{}
		d.repositoryTools = repositoryTools{}
		d.selected = id
		if tabs := d.projectTabs[id]; len(tabs) > 0 {
			for i := len(tabs) - 1; i >= 0; i-- {
				if tabs[i].loaded {
					d.activateProjectTab(tabs[i])
					break
				}
			}
		}
	}
	d.selected, d.path = id, root
}

func (d *Desktop) draftPath(id string) string {
	if d.draftDir == "" || id == "" || id == "." || id == ".." || strings.ContainsAny(id, "/\\:") {
		return ""
	}
	return filepath.Join(d.draftDir, id+".txt")
}

func (d *Desktop) loadDraft(id string) string {
	if message, ok := d.drafts[id]; ok {
		return message
	}
	if path := d.draftPath(id); path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data)
		}
		if !os.IsNotExist(err) {
			log.Printf("read commit draft: %v", err)
		}
	}
	return ""
}

func (d *Desktop) saveDraft() {
	if d.selected == "" || d.savedMessage == d.message {
		return
	}
	if d.drafts == nil {
		d.drafts = make(map[string]string)
	}
	d.drafts[d.selected] = d.message
	path := d.draftPath(d.selected)
	if path != "" {
		if err := writeDraft(path, d.message); err != nil {
			log.Printf("save commit draft: %v", err)
			return
		}
	}
	d.savedMessage = d.message
}

func writeDraft(path, message string) error {
	if message == "" {
		if err := os.Remove(path); !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".draft-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.WriteString(message)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
