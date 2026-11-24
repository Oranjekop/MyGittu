//go:build !windows

package ui

func setTaskbarIdentity(uintptr) error { return nil }
