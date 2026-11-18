package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
	native "github.com/egoist/mygo/ui"
	"mygittu/internal/ipc"
	"mygittu/internal/repository"
	"mygittu/internal/settings"
	"mygittu/internal/ui"
)

type remoteWindow struct {
	client   *ipc.Client
	win      *mygo.Window
	path     string
	action   string
	status   string
	err      string
	lang     string
	frame    int
	finished bool
}

func runRemote(client *ipc.Client, path, action string) error {
	if path == "" || (action != "fetch" && action != "pull" && action != "push") {
		return fmt.Errorf("invalid remote action")
	}
	d := &remoteWindow{client: client, path: path, action: action, lang: "zh-CN"}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	mygo.App.WhenReady(func() {
		d.win = mygo.NewWindow(mygo.WindowOptions{Title: "MyGittu", Width: 420, Height: 180, MinWidth: 360, MinHeight: 150, Content: native.View(d.view)})
		d.win.OnClosed(cancel)
		animationCtx, stopAnimation := context.WithCancel(ctx)
		go d.animate(animationCtx)
		go func() {
			defer stopAnimation()
			d.execute(ctx)
		}()
	})
	return mygo.App.Run()
}

func (d *remoteWindow) text(zh, en string) string {
	if d.lang == "en" {
		return en
	}
	return zh
}

func (d *remoteWindow) view(c *native.Context) {
	t := c.Theme()
	native.Column(c).Fill().Padding(22).Gap(12).Children(func() {
		native.Row(c).Gap(12).Children(func() {
			// Translation preserves the layout box, keeping the text still.
			ui.OperationIcon(c, 56, d.frame, d.action)
			native.Column(c).Gap(4).Children(func() {
				native.Text(c, d.text("MyGittu 远程操作", "MyGittu remote operation")).FontSize(18).Bold()
				native.Text(c, d.status).TextColor(t.TextMuted)
			})
		})
		if d.err != "" {
			native.Text(c, d.err).TextColor(native.Hex("#bf3030"))
		}
		if d.err != "" && native.PrimaryButton(c, d.text("关闭", "Close")).Clicked() {
			d.win.Close()
		}
	})
}

func (d *remoteWindow) animate(ctx context.Context) {
	ticker := time.NewTicker(time.Second / 24)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mygo.RunOnMain(func() {
				if !d.finished && !d.win.IsDestroyed() {
					d.frame = (d.frame + 1) % 24
					d.win.Invalidate()
				}
			})
		}
	}
}

func (d *remoteWindow) execute(ctx context.Context) {
	var prefs settings.Values
	_ = d.client.Call(ctx, "settings.get", ipc.Params{}, &prefs)
	d.win.Update(func() {
		if prefs.Language != "" {
			d.lang = prefs.Language
		}
		d.status = map[string]string{
			"fetch": d.text("正在获取远程更新…", "Fetching remote updates…"),
			"pull":  d.text("正在拉取远程更新…", "Pulling remote updates…"),
			"push":  d.text("正在推送本地提交…", "Pushing local commits…"),
		}[d.action]
	})
	var repo repository.Info
	if err := d.client.Call(ctx, "repo.register", ipc.Params{Path: d.path}, &repo); err != nil {
		d.fail(err)
		return
	}
	if err := d.client.Call(ctx, "git.remote", ipc.Params{RepoID: repo.ID, Action: d.action}, nil); err != nil {
		d.fail(err)
		return
	}
	d.win.Update(func() {
		d.finished, d.frame = true, 0
		d.status = d.text("已完成", "Completed")
	})
	time.AfterFunc(900*time.Millisecond, func() {
		mygo.RunOnMain(func() {
			if !d.win.IsDestroyed() {
				d.win.Close()
			}
		})
	})
}

func (d *remoteWindow) fail(err error) {
	d.win.Update(func() {
		d.finished, d.frame = true, 0
		d.status = d.text("操作失败", "Operation failed")
		d.err = strings.TrimSpace(err.Error())
	})
}
