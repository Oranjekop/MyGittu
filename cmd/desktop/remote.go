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
	fontSize                   float32
	client                     *ipc.Client
	win                        *mygo.Window
	path                       string
	action                     string
	status                     string
	err                        string
	lang                       string
	frame                      int
	finished                   bool
	captionStyled, captionDark bool
}

func runRemote(client *ipc.Client, path, action string) error {
	if path == "" || (action != "fetch" && action != "pull" && action != "push") {
		return fmt.Errorf("invalid remote action")
	}
	d := &remoteWindow{client: client, path: path, action: action, lang: "zh-CN", fontSize: 12.5}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	mygo.App.WhenReady(func() {
		d.win = mygo.NewWindow(mygo.WindowOptions{Title: "MyGittu", Width: 400, Height: 234, MinWidth: 360, MinHeight: 220, Content: native.View(d.view)})
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
	size := d.fontSize
	if size == 0 {
		size = 12.5
	}
	t := ui.ApplyTheme(c, size)
	if d.win != nil && (!d.captionStyled || d.captionDark != t.Dark) {
		if handle := d.win.NativeHandle(); handle != 0 {
			ui.StyleCaption(handle, t.Background, t.Text)
			d.captionStyled, d.captionDark = true, t.Dark
		}
	}
	if d.err == "" {
		message := d.status
		if message == "" {
			message = d.runningStatus()
		}
		native.Column(c).Fill().Background(t.Background).Padding(12, 12, 26, 12).Center().Gap(8).Children(func() {
			native.Image(c, d.rabbitFrame()).Width(116).Height(116)
			native.Text(c, message).FontSize(17 + t.FontSize - 12.5).Bold()
		})
		return
	}
	native.Column(c).Fill().Padding(18).Gap(12).Background(t.Background).Children(func() {
		native.Text(c, d.status).FontSize(17 + t.FontSize - 12.5).Bold().TextColor(t.Danger)
		native.Scroll(c).Grow(1).FillWidth().Background(t.Surface).Radius(8).Children(func() {
			native.Text(c, d.err).FillWidth().Padding(10)
		})
		native.Row(c).FillWidth().Children(func() {
			native.Box(c).Grow(1)
			if ui.IconButton(c, d.text("关闭", "Close"), "close", false).Clicked() {
				d.win.Close()
			}
		})
	})
}

func (d *remoteWindow) runningStatus() string {
	switch d.action {
	case "fetch":
		return d.text("正在获取远程更新…", "Fetching remote updates…")
	case "push":
		return d.text("正在推送本地提交…", "Pushing local commits…")
	default:
		return d.text("正在拉取远程更新…", "Pulling remote updates…")
	}
}

func (d *remoteWindow) animate(ctx context.Context) {
	ticker := time.NewTicker(remoteFrameInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mygo.RunOnMain(func() {
				if !d.finished && !d.win.IsDestroyed() {
					d.frame = (d.frame + 1) % 5
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
		d.fontSize = prefs.TextSize()
		if prefs.Language != "" {
			d.lang = prefs.Language
		}
		d.status = d.runningStatus()
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
		d.finished, d.frame = true, 3
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
