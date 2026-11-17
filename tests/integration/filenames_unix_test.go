//go:build !windows

package integration

import (
	"context"
	"encoding/json"
	"errors"
	git "mygittu/internal/git"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestUnixFilenameBytes(t *testing.T) {
	for _, test := range []struct{ name, path string }{{"newline", "line\nbreak.txt"}, {"non-utf8", string([]byte{'a', 0xff, 'b'})}} {
		t.Run(test.name, func(t *testing.T) {
			s, id, dir := setup(t)
			if e := os.WriteFile(filepath.Join(dir, test.path), []byte("content"), 0600); e != nil {
				if errors.Is(e, syscall.EILSEQ) {
					t.Skip("filesystem requires valid Unicode filenames")
				}
				t.Fatal(e)
			}
			ctx := context.Background()
			if e := s.Paths(ctx, id, []string{test.path}, true); e != nil {
				t.Fatal(e)
			}
			files, e := s.Status(ctx, id)
			if e != nil || len(files) != 1 {
				t.Fatalf("status: %+v %v", files, e)
			}
			b, e := json.Marshal(files)
			if e != nil {
				t.Fatal(e)
			}
			var restored []git.FileStatus
			if e = json.Unmarshal(b, &restored); e != nil {
				t.Fatal(e)
			}
			if restored[0].Path != test.path {
				t.Fatal("raw filename lost")
			}
		})
	}
}
