//go:build !windows

package main

func excludeRemoteFromTaskbar(uintptr) error { return nil }
