package integration

import (
	"context"
	"mygittu/internal/ipc"
	"mygittu/internal/repository"
	"mygittu/internal/status"
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestFramedAgentWorkflow(t *testing.T) {
	s, id, dir := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	server := &ipc.Server{Service: s, Cache: status.New(s)}
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	client := &ipc.Client{Dial: func(ctx context.Context) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", listener.Addr().String())
	}}
	call := func(method string, p ipc.Params, out any) {
		t.Helper()
		if e := client.Call(ctx, method, p, out); e != nil {
			t.Fatalf("%s: %v", method, e)
		}
	}
	call("health.ping", ipc.Params{}, nil)
	var repos []repository.Info
	call("repo.list", ipc.Params{}, &repos)
	if len(repos) != 1 {
		t.Fatal("missing registered repository")
	}
	write(t, filepath.Join(dir, "ipc.txt"), "over IPC")
	call("git.stage", ipc.Params{RepoID: id, Paths: []string{"ipc.txt"}}, nil)
	var hash string
	call("git.commit", ipc.Params{RepoID: id, Message: "IPC commit"}, &hash)
	if len(hash) != 40 {
		t.Fatal("commit missing")
	}
	var snap status.Snapshot
	call("status.refresh", ipc.Params{RepoID: id}, &snap)
	var overlay status.Overlay
	call("overlay.lookup", ipc.Params{Path: dir}, &overlay)
	if !overlay.Cached || overlay.Generation == 0 {
		t.Fatal("snapshot unavailable")
	}
	if e = client.Call(ctx, "shell.exec", ipc.Params{}, nil); e == nil {
		t.Fatal("arbitrary method accepted")
	}
	cancel()
	select {
	case e = <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("agent failed to stop")
	}
}
