package ui

import (
	"context"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"mygittu/internal/app"
	git "mygittu/internal/git"
	"mygittu/internal/ipc"
	"mygittu/internal/status"
)

func (d *Desktop) watchWorkspace(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mygo.RunOnMain(func() { d.pollWorkspace(ctx) })
		}
	}
}

func (d *Desktop) canPollWorkspace() bool {
	return d.result == "" && d.selected != "" && (d.page == 0 || d.page == 2) && !d.busy && !d.settingsOpen && !d.cloneOpen && !d.repositoriesOpen && !d.hunksOpen && !d.tools.conflictOpen && !d.tools.confirmOpen && !d.repositoryTools.confirmOpen && !d.repositoryTools.amendOpen
}

func (d *Desktop) pollWorkspace(ctx context.Context) {
	if ctx.Err() != nil || d.statusPolling || !d.canPollWorkspace() {
		return
	}
	d.statusPolling = true
	id, previous := d.selected, d.snapshot.Updated
	staged := d.staged
	var selected git.FileStatus
	if d.row >= 0 && d.row < len(d.changes) {
		selected = d.changes[d.row]
	}
	go func() {
		request, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
		defer cancel()
		var snapshot status.Snapshot
		err := d.Client.Call(request, "status.poll", ipc.Params{RepoID: id, StatusUpdated: previous}, &snapshot)
		var comparison app.Comparison
		var comparisonOK bool
		comparisonStaged := staged
		if err == nil && snapshot.Updated.After(previous) && selected.Path != "" {
			for _, file := range snapshot.Files {
				if file.Path == selected.Path && file.State != git.Ignored {
					comparisonStaged = workspaceDiffSide(file, staged)
					comparisonOK = d.Client.Call(request, "git.comparison", ipc.Params{RepoID: id, Path: file.Path, OriginalPath: file.OriginalPath, Staged: comparisonStaged}, &comparison) == nil
					break
				}
			}
		}
		mygo.RunOnMain(func() {
			d.statusPolling = false
			if ctx.Err() != nil || err != nil || !snapshot.Updated.After(previous) ||
				d.selected != id || !d.canPollWorkspace() || !d.snapshot.Updated.Equal(previous) {
				return
			}
			sameSide := d.staged == staged
			wasCherryPicking := d.snapshot.CherryPicking
			changed := d.applyWorkspaceStatus(snapshot)
			if d.action == "log" && wasCherryPicking && !snapshot.CherryPicking {
				d.page = 1
				d.refresh()
				d.win.Invalidate()
				return
			}
			if comparisonOK && sameSide && d.staged == comparisonStaged && d.row >= 0 && d.row < len(d.changes) && d.changes[d.row].Path == selected.Path {
				changed = d.applyComparison(selected.Path, comparison) || changed
			}
			if changed {
				d.win.Invalidate()
			}
		})
	}()
}

func (d *Desktop) applyWorkspaceStatus(snapshot status.Snapshot) bool {
	changes := make([]git.FileStatus, 0, len(snapshot.Files))
	staged := 0
	for _, file := range snapshot.Files {
		if file.State != git.Ignored {
			changes = append(changes, file)
			if file.Index != git.Clean {
				staged++
			}
		}
	}
	slices.SortFunc(changes, func(a, b git.FileStatus) int {
		aInFolder, bInFolder := strings.Contains(a.Path, "/"), strings.Contains(b.Path, "/")
		if aInFolder != bInFolder {
			if aInFolder {
				return -1
			}
			return 1
		}
		if order := strings.Compare(strings.ToLower(path.Base(a.Path)), strings.ToLower(path.Base(b.Path))); order != 0 {
			return order
		}
		return strings.Compare(a.Path, b.Path)
	})
	branchChanged := d.snapshot.Branch != snapshot.Branch || d.snapshot.Tracking != snapshot.Tracking || d.snapshot.Merging != snapshot.Merging || d.snapshot.CherryPicking != snapshot.CherryPicking
	if snapshot.CherryPicking && !d.snapshot.CherryPicking && slices.ContainsFunc(changes, func(file git.FileStatus) bool {
		return file.State == git.Conflicted
	}) {
		d.changesCollapsed = false
	}
	d.snapshot = snapshot
	if slices.Equal(d.changes, changes) {
		return branchChanged
	}
	var selected git.FileStatus
	if d.row >= 0 && d.row < len(d.changes) {
		selected = d.changes[d.row]
	}
	d.changes, d.stagedCount = changes, staged
	if len(d.changeTabs) > 0 {
		available := make(map[string]bool, len(changes))
		for _, file := range changes {
			available[file.Path] = true
		}
		d.changeTabs = slices.DeleteFunc(d.changeTabs, func(tab changeTab) bool {
			return !available[tab.path]
		})
	}
	d.row = slices.IndexFunc(changes, func(file git.FileStatus) bool { return file.Path == selected.Path })
	if d.row < 0 {
		d.comparison, d.binary = nil, false
		d.comparisonPath, d.comparisonLimited, d.comparisonError = "", false, ""
	} else {
		d.staged = workspaceDiffSide(changes[d.row], d.staged)
		if d.comparisonPath != changes[d.row].Path || d.comparisonStaged != d.staged {
			d.detail, d.comparison, d.binary = "", nil, false
			d.comparisonPath, d.comparisonLimited, d.comparisonError = "", false, ""
		}
	}
	return true
}
