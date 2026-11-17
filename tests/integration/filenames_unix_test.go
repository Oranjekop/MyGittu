//go:build !windows

package integration

import (
	"context"
	"encoding/json"
	git "mygittu/internal/git"
	"path/filepath"
	"testing"
)

func TestUnixFilenameBytes(t *testing.T) {
	s, id, dir := setup(t)
	ctx := context.Background()
	names := []string{"line\nbreak.txt", string([]byte{'a', 0xff, 'b'})}
	for _, name := range names {
		write(t, filepath.Join(dir, name), "content")
		if e := s.Paths(ctx, id, []string{name}, true); e != nil {
			t.Fatal(e)
		}
	}
	files, e := s.Status(ctx, id)
	if e != nil || len(files) != 2 {
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
	found := false
	for _, f := range restored {
		if f.Path == names[1] {
			found = true
		}
	}
	if !found {
		t.Fatal("raw filename lost")
	}
}
