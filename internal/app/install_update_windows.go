package app

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

func launchInstaller(path string) error {
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	shell := windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteW")
	result, _, _ := shell.Call(0, 0, uintptr(unsafe.Pointer(file)), 0, 0, 1)
	runtime.KeepAlive(file)
	if result <= 32 {
		return fmt.Errorf("could not open update installer (Windows error %d)", result)
	}
	return nil
}
