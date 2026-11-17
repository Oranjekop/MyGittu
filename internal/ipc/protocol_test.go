package ipc

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestFrames(t *testing.T) {
	in := Request{Version: 1, ID: "id", Method: "health.ping", Params: json.RawMessage(`{}`)}
	var b bytes.Buffer
	if e := WriteFrame(&b, in); e != nil {
		t.Fatal(e)
	}
	var out Request
	if e := ReadFrame(&b, &out); e != nil || out.ID != in.ID {
		t.Fatalf("roundtrip: %+v %v", out, e)
	}
	for _, size := range []uint32{0, MaxFrame + 1} {
		b.Reset()
		_ = binary.Write(&b, binary.BigEndian, size)
		if ReadFrame(&b, &out) == nil {
			t.Fatal("accepted invalid size")
		}
	}
	if ReadFrame(bytes.NewReader([]byte{0, 0, 0, 5, '{'}), &out) == nil {
		t.Fatal("accepted truncated frame")
	}
}
func TestVersionAndUnknownMethod(t *testing.T) {
	s := &Server{}
	for _, r := range []Request{{Version: 9, ID: "x"}, {Version: 1, ID: "x", Method: "shell.exec"}, {Version: 1, ID: "x", Params: json.RawMessage(`{`)}} {
		if s.Handle(context.Background(), r).OK {
			t.Fatal("accepted bad request")
		}
	}
}
func TestClientDeadlineAndCorrelation(t *testing.T) {
	for _, wrong := range []bool{false, true} {
		client := &Client{Dial: func(context.Context) (net.Conn, error) {
			a, b := net.Pipe()
			go func() {
				defer b.Close()
				var req Request
				_ = ReadFrame(b, &req)
				if wrong {
					_ = WriteFrame(b, Response{Version: 1, ID: "wrong", OK: true})
				} else {
					time.Sleep(100 * time.Millisecond)
				}
			}()
			return a, nil
		}}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		e := client.Call(ctx, "health.ping", Params{}, nil)
		cancel()
		if e == nil {
			t.Fatal("expected deadline or correlation error")
		}
	}
}
func FuzzFrame(f *testing.F) {
	f.Add([]byte{0, 0, 0, 2, '{', '}'})
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			return
		}
		var r Request
		_ = ReadFrame(bytes.NewReader(b), &r)
	})
}
