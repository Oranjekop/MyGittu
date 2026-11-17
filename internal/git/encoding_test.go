package git

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFilenameBytesRoundTrip(t *testing.T) {
	in := FileStatus{Path: string([]byte{'a', 0xff, 'b'}), OriginalPath: string([]byte{0xfe}), Index: Modified}
	b, e := json.Marshal(in)
	if e != nil {
		t.Fatal(e)
	}
	var out FileStatus
	if e = json.Unmarshal(b, &out); e != nil || out.Path != in.Path || out.OriginalPath != in.OriginalPath {
		t.Fatalf("lost bytes: %q %v", b, e)
	}
}
func TestRedaction(t *testing.T) {
	in := "failed https://user:secret@example.invalid/repo access_token=private password=hidden"
	out := Redact(in)
	for _, secret := range []string{"secret", "private", "hidden"} {
		if strings.Contains(out, secret) {
			t.Fatalf("leaked %s: %s", secret, out)
		}
	}
}
