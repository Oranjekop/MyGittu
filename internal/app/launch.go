package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func (s *Service) OpenDesktop(ctx context.Context, path, action string) error {
	switch action {
	case "status", "commit", "log", "diff", "clone", "pull", "push":
	default:
		return fmt.Errorf("unsupported UI action")
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("absolute repository path required")
	}
	if action != "clone" {
		repo, e := s.Registry.Discover(ctx, path)
		if e != nil {
			return e
		}
		path = repo.Root
	}
	self, e := os.Executable()
	if e != nil {
		return e
	}
	name := "desktop"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	cmd := exec.Command(filepath.Join(filepath.Dir(self), name), "--path", path, "--action", action)
	if e = cmd.Start(); e != nil {
		return e
	}
	return cmd.Process.Release()
}
