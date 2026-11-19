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
	diverged := d.text("remote.pull.diverged")
	d.remoteRunning, d.remoteNotice = action, ""
	d.task(func(ctx context.Context) (func(), error) {
		var result app.RemoteResult
		err := d.Client.Call(ctx, "git.remote", ipc.Params{RepoID: id, Action: action}, &result)
		return func() {
			d.remoteRunning = ""
			if d.selected != id {
				return
			}
			if retry {
				d.recordCommitPush(id, err)
				if err == nil && d.action == "commit" {
					d.result = d.text("commit.pushDone")
					return
				}
			}
			d.refresh()
			if err != nil {
				switch {
				case retry:
				case action == "pull" && pullDiverged(err):
					d.err = diverged
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
