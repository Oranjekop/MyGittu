package platform

import (
	"context"
	"fmt"
	"github.com/Microsoft/go-winio"
	"net"
	"os/user"
)

func endpoint() (string, string, error) {
	u, e := user.Current()
	if e != nil {
		return "", "", e
	}
	return `\\.\pipe\MyGittu-` + u.Uid, u.Uid, nil
}
func Listen() (net.Listener, error) {
	p, sid, e := endpoint()
	if e != nil {
		return nil, e
	}
	return winio.ListenPipe(p, &winio.PipeConfig{SecurityDescriptor: fmt.Sprintf("D:P(A;;GA;;;%s)", sid), MessageMode: false, InputBufferSize: 65536, OutputBufferSize: 65536})
}
func Dial(ctx context.Context) (net.Conn, error) {
	p, _, e := endpoint()
	if e != nil {
		return nil, e
	}
	return winio.DialPipeContext(ctx, p)
}
