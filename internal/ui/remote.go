package ui

import (
	"context"
	"fmt"
	"strings"

	native "github.com/egoist/mygo/ui"
	"mygittu/internal/app"
	"mygittu/internal/ipc"
)

func (d *Desktop) remoteAction(action string) {
	if d.selected == "" || d.busy {
		return
	}
	id := d.selected
	retry := action == "push" && d.pushRetryRepo == id
	completedCommit := action == "push" && d.action == "commit" && d.offerCommitPush
	diverged := d.text("remote.pull.diverged")
	d.remoteRunning, d.remoteNotice = action, ""
	d.remoteDiverged = false
	d.task(func(ctx context.Context) (func(), error) {
		var result app.RemoteResult
		err := d.Client.Call(ctx, "git.remote", ipc.Params{RepoID: id, Action: action}, &result)
		return func() {
			d.remoteRunning = ""
			if d.selected != id {
				return
			}
			if retry || completedCommit {
				d.recordCommitPush(id, err)
				if err == nil && d.action == "commit" {
					d.result = d.text("commit.pushDone")
					d.offerCommitPush = false
					return
				}
				if completedCommit {
					return
				}
			}
			d.refresh()
			if err != nil {
				switch {
				case retry:
				case action == "pull" && pullDiverged(err):
					d.err = diverged
					d.remoteDiverged = true
				default:
					d.err = err.Error()
				}
				return
			}
			key := "remote." + action + ".done"
			if result.UpToDate || result.NothingToPush {
				key = "remote." + action + ".unchanged"
			}
			d.remoteNotice = d.text(key)
		}, nil
	})
}

func (d *Desktop) compareUpstream() {
	id := d.selected
	d.task(func(ctx context.Context) (func(), error) {
		var pair [2]string
		if err := d.Client.Call(ctx, "git.compare.upstream", ipc.Params{RepoID: id}, &pair); err != nil {
			return nil, err
		}
		var comparison app.BranchComparisonSummary
		if err := d.Client.Call(ctx, "git.compare.summary", ipc.Params{RepoID: id, Name: pair[0], Query: pair[1]}, &comparison); err != nil {
			return nil, err
		}
		return func() {
			d.remoteDiverged = false
			d.page = 3
			d.tools.left, d.tools.right = pair[0], pair[1]
			d.tools.setComparison(comparison)
			d.tools.compareTab = 0
			d.refresh()
		}, nil
	})
}

func pullDiverged(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "not possible to fast-forward") || strings.Contains(message, "无法快进")
}

func (d *Desktop) recordCommitPush(id string, err error) {
	if err != nil {
		d.pushRetryRepo = id
		d.pushFailure = fmt.Sprintf(d.text("commit.pushFailed"), err)
	} else if d.pushRetryRepo == id {
		d.pushRetryRepo, d.pushFailure = "", ""
	}
}

func (d *Desktop) showCommitPushFailure(c *native.Context) {
	if d.pushFailure == "" || d.pushRetryRepo != d.selected {
		return
	}
	native.Row(c).FillWidth().AlignItems(native.Center).Gap(controlSpacing).Children(func() {
		native.Text(c, d.pushFailure).Grow(1).MinWidth(0).TextColor(c.Theme().Danger)
		if IconButton(c, d.text("remote.push.retry"), "push", false).Disabled(d.busy).Clicked() {
			d.remoteAction("push")
		}
	})
}
