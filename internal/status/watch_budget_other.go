//go:build !darwin

package status

type watchBudget struct{}

func newWatchBudget() watchBudget             { return watchBudget{} }
func (watchBudget) takeEntry() bool           { return true }
func (watchBudget) takeDirectory(string) bool { return true }
