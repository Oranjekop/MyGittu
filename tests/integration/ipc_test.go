package integration

import (
	"context"
	"encoding/json"
	"mygittu/internal/app"
	git "mygittu/internal/git"
	"mygittu/internal/ipc"
	"mygittu/internal/repository"
	"mygittu/internal/status"
	"net"
	"path/filepath"
	"slices"
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
	write(t, filepath.Join(dir, "new.txt"), "new file")
	var files []string
	call("git.files", ipc.Params{RepoID: id}, &files)
	if !slices.Contains(files, "ipc.txt") || !slices.Contains(files, "new.txt") {
		t.Fatalf("tracked or new file missing: %v", files)
	}
	var preview app.Comparison
	call("git.file.preview", ipc.Params{RepoID: id, Path: "ipc.txt"}, &preview)
	if preview.Binary || preview.New != "over IPC" {
		t.Fatalf("invalid project file preview: %+v", preview)
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

func TestUnopenedRepositoryStartsOverlayRefresh(t *testing.T) {
	s, _, _ := setup(t)
	other := t.TempDir()
	command(t, other, "init", "-b", "main")
	command(t, other, "config", "user.name", "Test")
	command(t, other, "config", "user.email", "test@example.invalid")
	command(t, other, "config", "commit.gpgsign", "false")
	file := filepath.Join(other, "README.md")
	write(t, file, "clean\n")
	command(t, other, "add", "README.md")
	command(t, other, "commit", "-m", "initial")

	server := &ipc.Server{Service: s, Cache: status.New(s)}
	params, err := json.Marshal(ipc.Params{Path: file})
	if err != nil {
		t.Fatal(err)
	}
	response := server.Handle(context.Background(), ipc.Request{
		Version: ipc.Version,
		ID:      "contains",
		Method:  "repo.contains",
		Params:  params,
	})
	if !response.OK {
		t.Fatalf("repo.contains failed: %+v", response.Error)
	}
	var result map[string]bool
	if err := json.Unmarshal(response.Result, &result); err != nil || !result["repository"] {
		t.Fatalf("unexpected repo.contains result: %s", response.Result)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := s.Registry.FindPath(file); ok {
			overlay := server.Cache.Lookup(file)
			if overlay.Cached && overlay.State == git.Clean {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("unopened repository did not produce a cached clean overlay")
}
