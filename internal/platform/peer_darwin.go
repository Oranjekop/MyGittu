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
		cred, err := unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		allowed = err == nil && cred.Uid == uint32(os.Getuid())
	})
	return e == nil && allowed
}
