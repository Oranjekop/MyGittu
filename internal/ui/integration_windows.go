package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

const explorerSupported = true

func modernSupported() bool { return windows.RtlGetVersion().BuildNumber >= 22000 }

func runIntegrationScript(ctx context.Context, action string, enabled bool) ([]byte, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	system, err := windows.GetSystemDirectory()
	if err != nil {
		return nil, err
	}
	script := filepath.Join(filepath.Dir(exe), "integration.ps1")
	if _, err := os.Stat(script); err != nil {
		return nil, fmt.Errorf("Windows integration requires an installed copy of MyGittu")
	}
	state := "Off"
	if enabled {
		state = "On"
	}
	cmd := exec.CommandContext(ctx, filepath.Join(system, "WindowsPowerShell", "v1.0", "powershell.exe"), "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script, "-Action", action, "-State", state)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if failed, ok := err.(*exec.ExitError); ok {
			detail := strings.TrimSpace(string(failed.Stderr))
			if len(detail) > 1200 {
				detail = detail[:1200]
			}
			if detail != "" {
				return nil, fmt.Errorf("%s", detail)
			}
		}
		return nil, err
	}
	return out, nil
}

func readIntegration(ctx context.Context) (integrationState, error) {
	var state integrationState
	output, err := runIntegrationScript(ctx, "Status", false)
	if err == nil {
		err = json.Unmarshal(output, &state)
	}
	return state, err
}

func changeIntegration(ctx context.Context, action string, enabled bool) error {
	_, err := runIntegrationScript(ctx, action, enabled)
	return err
}
