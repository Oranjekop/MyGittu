package platform

import (
	"path/filepath"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var shellChangeNotify = windows.NewLazySystemDLL("shell32.dll").NewProc("SHChangeNotify")

func NotifyRepositoryOverlay(root string) {
	const updateDir = 0x1000
	const pathW = 0x0005
	for _, dir := range []string{filepath.Dir(root), root} {
		name, err := windows.UTF16PtrFromString(dir)
		if err == nil {
			shellChangeNotify.Call(updateDir, pathW, uintptr(unsafe.Pointer(name)), 0)
			runtime.KeepAlive(name)
		}
	}
}
