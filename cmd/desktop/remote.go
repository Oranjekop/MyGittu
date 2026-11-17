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
	client *ipc.Client
	win    *mygo.Window
	path   string
	action string
	status string
	err    string
	lang   string
	frame  int
}

func runRemote(client *ipc.Client, path, action string) error {
	if path == "" || (action != "fetch" && action != "pull" && action != "push") {
		return fmt.Errorf("invalid remote action")
	}
	d := &remoteWindow{client: client, path: path, action: action, lang: "zh-CN"}
	mygo.App.WhenReady(func() {
		d.win = mygo.NewWindow(mygo.WindowOptions{Title: "MyGittu", Width: 420, Height: 180, MinWidth: 360, MinHeight: 150, Content: native.View(d.view)})
		go d.animate()
		go d.execute()
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
			bounce := float32(44 + (d.frame%3)*3)
			native.Image(c, ui.RemoteBitmap()).Width(bounce).Height(bounce)
			native.Column(c).Gap(4).Children(func() {
				native.Text(c, d.text("MyGittu 远程操作", "MyGittu remote operation")).FontSize(18).Bold()
				native.Text(c, d.status+strings.Repeat(".", d.frame%4)).TextColor(t.TextMuted)
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

func (d *remoteWindow) animate() {
	ticker := time.NewTicker(180 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		if d.win == nil || d.err != "" {
			return
		}
		d.frame++
		mygo.RunOnMain(func() { d.win.Invalidate() })
	}
}

func (d *remoteWindow) execute() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var prefs settings.Values
	if err := d.client.Call(ctx, "settings.get", ipc.Params{}, &prefs); err == nil && prefs.Language != "" {
		d.lang = prefs.Language
	}
	d.status = map[string]string{
		"fetch": d.text("正在获取远程更新…", "Fetching remote updates…"),
		"pull":  d.text("正在拉取远程更新…", "Pulling remote updates…"),
		"push":  d.text("正在推送本地提交…", "Pushing local commits…"),
	}[d.action]
	mygo.RunOnMain(func() { d.win.Invalidate() })
	var repo repository.Info
	if err := d.client.Call(ctx, "repo.register", ipc.Params{Path: d.path}, &repo); err != nil {
		d.fail(err)
		return
	}
	if err := d.client.Call(ctx, "git.remote", ipc.Params{RepoID: repo.ID, Action: d.action}, nil); err != nil {
		d.fail(err)
		return
	}
	d.status = d.text("已完成", "Completed")
	mygo.RunOnMain(func() { d.win.Invalidate(); time.AfterFunc(900*time.Millisecond, d.win.Close) })
}

func (d *remoteWindow) fail(err error) {
	d.err = strings.TrimSpace(err.Error())
	mygo.RunOnMain(func() { d.win.Invalidate() })
}
