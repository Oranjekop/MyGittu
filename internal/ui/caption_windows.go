package ui

import (
	"unsafe"

	native "github.com/egoist/mygo/ui"
	"golang.org/x/sys/windows"
)

var setWindowAttribute = windows.NewLazySystemDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")

func styleCaption(handle uintptr, background, foreground native.Color) {
	const captionColor, captionText = 35, 36
	for _, item := range []struct {
		attribute uintptr
		color     native.Color
	}{{captionColor, background}, {captionText, foreground}} {
		rgb := uint32(item.color.R) | uint32(item.color.G)<<8 | uint32(item.color.B)<<16
		// Older Windows builds keep the system caption colors.
		setWindowAttribute.Call(handle, item.attribute, uintptr(unsafe.Pointer(&rgb)), unsafe.Sizeof(rgb))
	}
}
