package status

import (
	"os"

	"golang.org/x/sys/unix"
)

// kqueue opens a descriptor for the directory and each immediate child.
// Reserve headroom for IPC, Git processes, and directory reads.
type watchBudget struct{ remaining int }

func newWatchBudget() watchBudget {
	var limit unix.Rlimit
	if unix.Getrlimit(unix.RLIMIT_NOFILE, &limit) != nil {
		return watchBudget{remaining: 96}
	}
	return watchBudget{remaining: int(min(limit.Cur/2, 1024))}
}

func (b *watchBudget) takeEntry() bool {
	b.remaining--
	return b.remaining >= 0
}

func (b *watchBudget) takeDirectory(path string) bool {
	if !b.takeEntry() {
		return false
	}
	dir, err := os.Open(path)
	if err != nil {
		return !descriptorLimit(err)
	}
	defer dir.Close()

	// Read at most one entry beyond the remaining budget.
	entries, err := dir.ReadDir(b.remaining + 1)
	b.remaining -= len(entries)
	return b.remaining >= 0 && !descriptorLimit(err)
}
