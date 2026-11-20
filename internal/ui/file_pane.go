package ui

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	native "github.com/egoist/mygo/ui"
	"mygittu/internal/platform"
)

const (
	minFilePaneWidth        = 284
	defaultFilePaneWidth    = 310
	defaultProjectPaneWidth = 342
)

func filePanePath(window string) string {
	dir, err := platform.ConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "windows", window+"-file-pane.json")
}

func restoreFilePane(path string, target *float32) {
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var width float32
	if json.Unmarshal(data, &width) == nil && width >= minFilePaneWidth {
		*target = width
	}
}

func saveFilePane(path string, width float32, changed bool) {
	if path == "" || !changed {
		return
	}
	data, err := json.Marshal(width)
	if err == nil {
		err = writeWindowPreference(path, data)
	}
	if err != nil {
		log.Printf("save file pane width: %v", err)
	}
}

func filePaneWidth(available, preferred float32) float32 {
	return min(max(preferred, minFilePaneWidth), max(minFilePaneWidth, available-320))
}

func splitFilePanes(c *native.Context, t *native.Theme, width *float32, reset float32, left, right func()) bool {
	dividerTheme := *t
	dividerTheme.Border = t.Surface
	dividerTheme.Accent = t.Border
	pane := func(right bool, contents func()) {
		container := native.Themed(c, t).Fill().Background(t.Surface)
		if right {
			container.PaddingStart(panelSpacing / 2)
		} else {
			container.PaddingEnd((panelSpacing - 1) / 2)
		}
		container.Children(contents)
	}
	changed := false
	native.Themed(c, &dividerTheme).Grow(1).FillWidth().Background(t.Surface).Children(func() {
		changed = native.Split(c, width, func() { pane(false, left) }, func() { pane(true, right) }).
			Grow(1).FillWidth().SplitReset(reset).Changed()
	})
	return changed
}
