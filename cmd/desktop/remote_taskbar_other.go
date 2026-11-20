//go:build !windows

package main

func setRemoteTaskbarIdentity(uintptr) error { return nil }

func excludeRemoteFromTaskbar(uintptr) error { return nil }
