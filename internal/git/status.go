package git

import (
	"bytes"
	"fmt"
)

type State string

const (
	Clean      State = "clean"
	Modified   State = "modified"
	Added      State = "added"
	Deleted    State = "deleted"
	Untracked  State = "untracked"
	Ignored    State = "ignored"
	Conflicted State = "conflicted"
	Unknown    State = "unknown"
)

type FileStatus struct {
	Path         string `json:"path"`
	OriginalPath string `json:"original_path,omitempty"`
	Index        State  `json:"index"`
	Worktree     State  `json:"worktree"`
	State        State  `json:"state"`
	Submodule    string `json:"submodule,omitempty"`
}

func decode(c byte) State {
	switch c {
	case '.':
		return Clean
	case 'M', 'T', 'R', 'C':
		return Modified
	case 'A':
		return Added
	case 'D':
		return Deleted
	case 'U':
		return Conflicted
	}
	return Unknown
}
func Priority(s State) int {
	switch s {
	case Conflicted:
		return 8
	case Deleted:
		return 7
	case Modified:
		return 6
	case Added:
		return 5
	case Untracked:
		return 4
	case Unknown:
		return 3
	case Ignored:
		return 2
	case Clean:
		return 1
	}
	return 0
}
func Aggregate(a, b State) State {
	if Priority(a) >= Priority(b) {
		return a
	}
	return b
}
func ParseStatus(data []byte) ([]FileStatus, error) {
	result := make([]FileStatus, 0)
	if len(data) == 0 {
		return result, nil
	}
	if data[len(data)-1] != 0 {
		return nil, fmt.Errorf("unterminated porcelain record")
	}
	records := bytes.Split(data[:len(data)-1], []byte{0})
	for i := 0; i < len(records); i++ {
		line := records[i]
		if len(line) < 2 {
			return nil, fmt.Errorf("short porcelain record")
		}
		f := FileStatus{Index: Clean, Worktree: Clean}
		switch line[0] {
		case '#':
			continue
		case '?', '!':
			if len(line) < 3 || line[1] != ' ' {
				return nil, fmt.Errorf("invalid path record")
			}
			f.Path = string(line[2:])
			f.Worktree = Untracked
			if line[0] == '!' {
				f.Worktree = Ignored
			}
			f.State = f.Worktree
		case '1', '2', 'u':
			n := 9
			if line[0] == '2' {
				n = 10
			}
			if line[0] == 'u' {
				n = 11
			}
			parts := bytes.SplitN(line, []byte{' '}, n)
			if len(parts) != n || len(parts[1]) != 2 || len(parts[n-1]) == 0 {
				return nil, fmt.Errorf("invalid porcelain %c record", line[0])
			}
			f.Path = string(parts[n-1])
			f.Submodule = string(parts[2])
			f.Index = decode(parts[1][0])
			f.Worktree = decode(parts[1][1])
			f.State = Aggregate(f.Index, f.Worktree)
			if line[0] == 'u' {
				f.Index = Conflicted
				f.Worktree = Conflicted
				f.State = Conflicted
			}
			if line[0] == '2' {
				i++
				if i >= len(records) || len(records[i]) == 0 {
					return nil, fmt.Errorf("missing rename source")
				}
				f.OriginalPath = string(records[i])
			}
		default:
			return nil, fmt.Errorf("unknown porcelain record")
		}
		result = append(result, f)
	}
	return result, nil
}
