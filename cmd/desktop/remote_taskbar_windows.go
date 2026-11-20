package main

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var remoteWindowPropertyStore = windows.NewLazySystemDLL("shell32.dll").NewProc("SHGetPropertyStoreForWindow")

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

type remotePropertyKey struct {
	format windows.GUID
	id     uint32
}

type remoteStringProperty struct {
	kind     uint16
	reserved [3]uint16
	value    *uint16
	padding  uintptr
}

type remotePropertyStore struct {
	methods *struct {
		queryInterface, addRef, release uintptr
		getCount, getAt, getValue       uintptr
		setValue, commit                uintptr
	}
}

func setRemoteTaskbarIdentity(hwnd uintptr) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	iid := windows.GUID{Data1: 0x886d8eeb, Data2: 0x8cf2, Data3: 0x4446, Data4: [8]byte{0x8d, 0x02, 0xcd, 0xba, 0x1d, 0xbd, 0xcf, 0x99}}
	var store *remotePropertyStore
	result, _, _ := remoteWindowPropertyStore.Call(hwnd, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&store)))
	if int32(result) < 0 {
		return fmt.Errorf("SHGetPropertyStoreForWindow: %#08x", uint32(result))
	}
	this := uintptr(unsafe.Pointer(store))
	defer syscall.SyscallN(store.methods.release, this)

	appUserModel := windows.GUID{Data1: 0x9f4c2855, Data2: 0x9f79, Data3: 0x4b39, Data4: [8]byte{0xa8, 0xd0, 0xe1, 0xd4, 0x2d, 0xe1, 0xd5, 0xf3}}
	// Explorer refreshes the taskbar entry when the ID is assigned.
	// Publish its icon and relaunch details before assigning the ID.
	for _, property := range []struct {
		id    uint32
		value string
	}{
		{2, `"` + exe + `"`},            // RelaunchCommand
		{3, exe + ",0"},                 // RelaunchIconResource
		{4, "MyGittu"},                  // RelaunchDisplayNameResource
		{5, "Oranjekop.MyGittu.Remote"}, // ID
	} {
		key := remotePropertyKey{format: appUserModel, id: property.id}
		value, err := windows.UTF16PtrFromString(property.value)
		if err != nil {
			return err
		}
		const vtLPWSTR = 31
		variant := remoteStringProperty{kind: vtLPWSTR, value: value}
		result, _, _ = syscall.SyscallN(store.methods.setValue, this, uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&variant)))
		runtime.KeepAlive(variant)
		if int32(result) < 0 {
			return fmt.Errorf("set taskbar property %d: %#08x", property.id, uint32(result))
		}
	}
	result, _, _ = syscall.SyscallN(store.methods.commit, this)
	runtime.KeepAlive(store)
	if int32(result) < 0 {
		return fmt.Errorf("commit taskbar properties: %#08x", uint32(result))
	}
	return nil
}
