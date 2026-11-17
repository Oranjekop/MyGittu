package main

import (
	"os/exec"
	"syscall"
)

func agentName() string { return "agent.exe" }
func configureAgent(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
