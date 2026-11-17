//go:build !windows

package platform

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
)

func socketPath() (string, error) {
	dir, e := ConfigDir()
	if e != nil {
		return "", e
	}
	if e = os.MkdirAll(dir, 0700); e != nil {
		return "", e
	}
	st, e := os.Lstat(dir)
	if e != nil {
		return "", e
	}
	owner, ok := st.Sys().(*syscall.Stat_t)
	if !st.IsDir() || st.Mode().Perm()&0077 != 0 || !ok || owner.Uid != uint32(os.Getuid()) {
		return "", fmt.Errorf("insecure socket directory")
	}
	return filepath.Join(dir, "agent.sock"), nil
}
func Listen() (net.Listener, error) {
	p, e := socketPath()
	if e != nil {
		return nil, e
	}
	if _, e = os.Lstat(p); e == nil {
		c, err := net.Dial("unix", p)
		if err == nil {
			c.Close()
			return nil, fmt.Errorf("agent already running")
		}
		if e = os.Remove(p); e != nil {
			return nil, e
		}
	}
	l, e := net.Listen("unix", p)
	if e == nil {
		e = os.Chmod(p, 0600)
		if e != nil {
			l.Close()
		}
	}
	if e != nil {
		return nil, e
	}
	return &privateListener{l}, nil
}

type privateListener struct{ net.Listener }

func (l *privateListener) Accept() (net.Conn, error) {
	for {
		conn, e := l.Listener.Accept()
		if e != nil {
			return nil, e
		}
		if sameUser(conn) {
			return conn, nil
		}
		conn.Close()
	}
}
func Dial(ctx context.Context) (net.Conn, error) {
	p, e := socketPath()
	if e != nil {
		return nil, e
	}
	return (&net.Dialer{}).DialContext(ctx, "unix", p)
}
