package ipc

import (
	"encoding/json"
	"testing"
)

func TestPathParametersPreserveBytes(t *testing.T) {
	in := Params{Path: string([]byte{'p', 255}), Paths: []string{"valid", string([]byte{254, 'x'})}}
	b, e := json.Marshal(in)
	if e != nil {
		t.Fatal(e)
	}
	var out Params
	if e = json.Unmarshal(b, &out); e != nil || out.Path != in.Path || len(out.Paths) != 2 || out.Paths[1] != in.Paths[1] {
		t.Fatalf("lost raw path: %q %v", b, e)
	}
}
