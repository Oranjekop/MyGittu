package platform

import (
	"path/filepath"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var shellChangeNotify = windows.NewLazySystemDLL("shell32.dll").NewProc("SHChangeNotify")

func NotifyRepositoryOverlay(root string, paths []string, broad bool) {
	const (
		updateDir  = 0x1000
		updateItem = 0x2000
	)
	const pathW = 0x0005
	const flushNoWait = 0x2000
	notify := func(event uintptr, path string) {
		name, err := windows.UTF16PtrFromString(path)
		if err == nil {
			shellChangeNotify.Call(event, pathW|flushNoWait, uintptr(unsafe.Pointer(name)), 0)
			runtime.KeepAlive(name)
		}
	}
	if broad {
		notify(updateItem, root)
		notify(updateDir, filepath.Dir(root))
		notify(updateDir, root)
		for _, path := range paths {
			if path != "." {
				notify(updateDir, filepath.Join(root, path))
			}
		}
		return
	}
	dirs := make(map[string]struct{})
	for _, path := range paths {
		item := filepath.Join(root, path)
		notify(updateItem, item)
		dirs[filepath.Dir(item)] = struct{}{}
	}
	for dir := range dirs {
		notify(updateDir, dir)
	}
}
