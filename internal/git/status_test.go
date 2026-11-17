package git

import (
	"strings"
	"testing"
)

func TestPorcelainSpecialPaths(t *testing.T) {
	data := "1 MM N... 100644 100644 100644 a b 中文 name\n.txt\x00" + "2 R. N... 100644 100644 100644 a b R100 new name\x00old\nname\x00" + "u UU N... 100644 100644 100644 100644 a b c conflict\x00? --literal [*].txt\x00! ignored/\x00"
	files, e := ParseStatus([]byte(data))
	if e != nil {
		t.Fatal(e)
	}
	if len(files) != 5 || files[0].Path != "中文 name\n.txt" || files[0].Index != Modified || files[0].Worktree != Modified {
		t.Fatalf("bad status: %+v", files)
	}
	if files[1].OriginalPath != "old\nname" || files[2].State != Conflicted || files[3].State != Untracked || files[4].State != Ignored {
		t.Fatalf("bad record semantics: %+v", files)
	}
}
func TestMalformedStatus(t *testing.T) {
	for _, s := range []string{"? no nul", "2 R. N... 1 1 1 a b R100 new\x00", "1 short\x00", "x unknown\x00", "? \x00"} {
		_, e := ParseStatus([]byte(s))
		if e == nil {
			t.Errorf("accepted %q", s)
		}
	}
}
func FuzzStatus(f *testing.F) {
	f.Add([]byte("? hello\x00"))
	f.Add([]byte("1 .M N... 100644 100644 100644 a b path\x00"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			return
		}
		files, e := ParseStatus(b)
		if e == nil {
			for _, v := range files {
				if strings.ContainsRune(v.Path, 0) {
					t.Fatal("NUL in path")
				}
			}
		}
	})
}
