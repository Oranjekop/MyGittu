//go:build !windows

package app

import "os/exec"

func launchInstaller(path string) error {
	return exec.Command("/usr/bin/open", path).Run()
}
