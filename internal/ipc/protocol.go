package ipc

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

const Version = 1
const MaxFrame = 8 << 20

type Request struct {
	Version int             `json:"version"`
	ID      string          `json:"request_id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}
type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type Response struct {
	Version int             `json:"version"`
	ID      string          `json:"request_id"`
	OK      bool            `json:"ok"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Failure        `json:"error,omitempty"`
}

func ReadFrame(r io.Reader, v any) error {
	var n uint32
	if e := binary.Read(r, binary.BigEndian, &n); e != nil {
		return e
	}
	if n == 0 || n > MaxFrame {
		return fmt.Errorf("invalid frame size")
	}
	b := make([]byte, n)
	if _, e := io.ReadFull(r, b); e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}
func WriteFrame(w io.Writer, v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	if len(b) > MaxFrame {
		return fmt.Errorf("frame too large")
	}
	var h [4]byte
	binary.BigEndian.PutUint32(h[:], uint32(len(b)))
	for _, part := range [][]byte{h[:], b} {
		for len(part) > 0 {
			n, e := w.Write(part)
			if e != nil {
				return e
			}
			if n == 0 {
				return io.ErrShortWrite
			}
			part = part[n:]
		}
	}
	return nil
}
