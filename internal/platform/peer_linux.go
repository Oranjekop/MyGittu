package platform

import (
	"golang.org/x/sys/unix"
	"net"
	"os"
)

func sameUser(c net.Conn) bool {
	u, ok := c.(*net.UnixConn)
	if !ok {
		return false
	}
	raw, e := u.SyscallConn()
	if e != nil {
		return false
	}
	allowed := false
	e = raw.Control(func(fd uintptr) {
		cred, err := unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
		allowed = err == nil && cred.Uid == uint32(os.Getuid())
	})
	return e == nil && allowed
}
