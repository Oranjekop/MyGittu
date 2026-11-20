package platform

import (
	"path/filepath"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var shellChangeNotify = windows.NewLazySystemDLL("shell32.dll").NewProc("SHChangeNotify")

func NotifyRepositoryOverlay(root string) {
	const (
		updateDir  = 0x1000
		updateItem = 0x2000
	)
	const pathW = 0x0005
	for _, change := range []struct {
		event uintptr
		path  string
	}{{updateItem, root}, {updateDir, filepath.Dir(root)}, {updateDir, root}} {
		name, err := windows.UTF16PtrFromString(change.path)
		if err == nil {
			shellChangeNotify.Call(change.event, pathW, uintptr(unsafe.Pointer(name)), 0)
			runtime.KeepAlive(name)
		}
	}
}
