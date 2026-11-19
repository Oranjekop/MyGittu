package platform

import (
	"os"
	"path/filepath"
)

func ConfigDir() (string, error) {
	p, e := os.UserConfigDir()
	return filepath.Join(p, "MyGittu"), e
}
