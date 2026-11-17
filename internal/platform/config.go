package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// App Group is supplied by the signed macOS bundle. Both the Agent and Finder
// extension use this directory; ordinary development uses per-user app data.
func ConfigDir() (string, error) {
	if runtime.GOOS == "darwin" {
		if group := os.Getenv("MYGITTU_APP_GROUP"); group != "" {
			if strings.ContainsAny(group, "/\\") || !strings.Contains(group, ".") {
				return "", fmt.Errorf("invalid app group")
			}
			home, e := os.UserHomeDir()
			if e != nil {
				return "", e
			}
			return filepath.Join(home, "Library", "Group Containers", group), nil
		}
	}
	p, e := os.UserConfigDir()
	return filepath.Join(p, "MyGittu"), e
}
