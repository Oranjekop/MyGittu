package main

import (
	"fmt"
	"golang.org/x/sys/windows"
)

// SkipTaskbar removes an already-created taskbar button in MyGo on Windows.
// Apply the native style while the HWND is still hidden to avoid that flash.
func excludeRemoteFromTaskbar(hwnd uintptr) error {
	const (
		extendedStyle = ^uintptr(19) // GWL_EXSTYLE (-20).
		toolWindow    = 0x80
		appWindow     = 0x40000
	)
	user32 := windows.NewLazySystemDLL("user32.dll")
	getStyle := user32.NewProc("GetWindowLongW")
	setStyle := user32.NewProc("SetWindowLongW")
	style, _, _ := getStyle.Call(hwnd, extendedStyle)
	_, _, err := setStyle.Call(hwnd, extendedStyle, style&^appWindow|toolWindow)
	style, _, _ = getStyle.Call(hwnd, extendedStyle)
	if style&(toolWindow|appWindow) != toolWindow {
		return fmt.Errorf("set pull window extended style: %w", err)
	}
	return nil
}
