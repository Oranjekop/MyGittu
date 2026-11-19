package platform

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// appGroup is embedded in all executables in the signed macOS bundle.
var appGroup string

var socketDir = sync.OnceValues(func() (string, error) {
	if appGroup == "" {
		return ConfigDir()
	}
	if _, err := purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL); err != nil {
		return "", err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))

	group := objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), appGroup)
	manager := objc.ID(objc.GetClass("NSFileManager")).Send(objc.RegisterName("defaultManager"))
	url := manager.Send(objc.RegisterName("containerURLForSecurityApplicationGroupIdentifier:"), group)
	if url == 0 {
		return "", fmt.Errorf("cannot access macOS App Group %q", appGroup)
	}
	path := url.Send(objc.RegisterName("path"))
	return filepath.Join(objc.Send[string](path, objc.RegisterName("UTF8String")), "ipc"), nil
})
