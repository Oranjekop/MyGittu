//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

func agentName() string          { return "agent" }
func configureAgent(c *exec.Cmd) { c.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
