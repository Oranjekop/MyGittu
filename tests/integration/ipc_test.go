package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"math/rand/v2"
	"mygittu/internal/app"
	git "mygittu/internal/git"
	"mygittu/internal/ipc"
	"mygittu/internal/repository"
	"mygittu/internal/status"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
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
	write(t, filepath.Join(dir, "ipc.txt"), "unstaged content")
	var stagedPatch string
	call("git.diff.staged-for-message", ipc.Params{RepoID: id}, &stagedPatch)
	if !strings.Contains(stagedPatch, "+over IPC") || strings.Contains(stagedPatch, "unstaged content") {
		t.Fatal("incorrect content in staged AI patch")
	}
	write(t, filepath.Join(dir, "ipc.txt"), "over IPC")
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

func TestLargeFilePreviewsThroughAgent(t *testing.T) {
	service, id, dir := setup(t)
	server := &ipc.Server{Service: service}
	preview := func(name string, data []byte, offset int64, version string) app.FilePreview {
		t.Helper()
		if data != nil {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		params, err := json.Marshal(ipc.Params{RepoID: id, Path: name, PreviewOffset: offset, PreviewVersion: version})
		if err != nil {
			t.Fatal(err)
		}
		response := server.Handle(context.Background(), ipc.Request{
			Version: ipc.Version, ID: "preview", Method: "git.file.preview", Params: params,
		})
		if !response.OK {
			t.Fatalf("preview %s: %+v", name, response.Error)
		}
		var frame bytes.Buffer
		if err := ipc.WriteFrame(&frame, response); err != nil {
			t.Fatal(err)
		}
		var decoded ipc.Response
		if err := ipc.ReadFrame(&frame, &decoded); err != nil {
			t.Fatal(err)
		}
		var result app.FilePreview
		if err := json.Unmarshal(decoded.Result, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}

	src := image.NewNRGBA(image.Rect(0, 0, 1600, 900))
	rng := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < len(src.Pix); i += 4 {
		rgb := rng.Uint32()
		src.Pix[i], src.Pix[i+1], src.Pix[i+2], src.Pix[i+3] = byte(rgb), byte(rgb>>8), byte(rgb>>16), 255
	}
	var original bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&original, src); err != nil {
		t.Fatal(err)
	}
	if original.Len() <= 256<<10 {
		t.Fatal("fixture must exceed the old preview limit")
	}
	// Content detection works even with a misleading extension.
	thumb := preview("large.dat", original.Bytes(), 0, "")
	size, err := png.DecodeConfig(bytes.NewReader(thumb.ImagePNG))
	if err != nil || size.Width != 1024 || size.Height != 576 || thumb.Binary {
		t.Fatalf("expected a 1024x576 thumbnail, got %+v, %v", size, err)
	}
	largeText := strings.Repeat("a", (256<<10)-1) + "汉字"
	text := preview("large.txt", []byte(largeText), 0, "")
	if text.NextOffset != (256<<10)-1 || text.Binary || !utf8.ValidString(text.New) {
		t.Fatal("expected valid text ending before the split UTF-8 character")
	}
	last := preview("large.txt", nil, text.NextOffset, text.Version)
	if last.NextOffset != 0 || text.New+last.New != largeText {
		t.Fatal("byte-limited pages did not cover the complete text")
	}
	if firstAgain := preview("large.txt", nil, 0, last.Version); firstAgain.New != text.New || firstAgain.NextOffset != text.NextOffset {
		t.Fatal("the first page changed when navigating backward")
	}

	lineContent := strings.Repeat("line\r\n", 9000)
	manyLines := preview("many-lines.txt", []byte(lineContent), 0, "")
	if manyLines.NextOffset != int64(len("line\r\n")*8000) || strings.Count(manyLines.New, "\n") != 8000 {
		t.Fatal("expected the first 8000 lines")
	}
	lineTail := preview("many-lines.txt", nil, manyLines.NextOffset, manyLines.Version)
	if lineTail.NextOffset != 0 || manyLines.New+lineTail.New != lineContent {
		t.Fatal("line-limited pages lost content")
	}

	if err := os.WriteFile(filepath.Join(dir, "large.txt"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.PreviewFile(context.Background(), id, "large.txt", text.NextOffset, text.Version); err == nil {
		t.Fatal("server accepted a cursor from before the file changed")
	}
}
