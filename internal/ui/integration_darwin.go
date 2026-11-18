//go:build darwin

package ui

import (
	"bytes"
	"context"
	"encoding/xml"
	"os"
	"path/filepath"
)

const explorerSupported = false

func modernSupported() bool { return false }

func loginAgentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", "dev.mygittu.desktop.login.plist"), nil
}

func readIntegration(context.Context) (integrationState, error) {
	path, err := loginAgentPath()
	if err != nil {
		return integrationState{}, err
	}
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		return integrationState{}, nil
	}
	return integrationState{Startup: err == nil}, err
}

func changeIntegration(_ context.Context, _ string, enabled bool) error {
	path, err := loginAgentPath()
	if err != nil {
		return err
	}
	if !enabled {
		err = os.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	app := filepath.Clean(filepath.Join(filepath.Dir(exe), "..", ".."))
	if filepath.Ext(app) != ".app" {
		return &os.PathError{Op: "locate application bundle", Path: app, Err: os.ErrInvalid}
	}
	var escaped bytes.Buffer
	if err := xml.EscapeText(&escaped, []byte(app)); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>Label</key><string>dev.mygittu.desktop.login</string>
<key>ProgramArguments</key><array><string>/usr/bin/open</string><string>` + escaped.String() + `</string></array>
<key>RunAtLoad</key><true/>
</dict></plist>`)
	tmp, err := os.CreateTemp(filepath.Dir(path), ".mygittu-login-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, writeErr := tmp.Write(data)
	closeErr := tmp.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp.Name(), path)
}
