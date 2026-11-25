package main

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"
	native "github.com/egoist/mygo/ui"
	"mygittu/internal/app"
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
	upToDate                   bool
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
		d.win = ui.NewRememberedWindow(d.windowOptions())
		handle := d.win.NativeHandle()
		if d.hidesTaskbar() {
			if err := excludeRemoteFromTaskbar(handle); err != nil {
				fmt.Printf("exclude MyGittu pull window from taskbar: %v\n", err)
			}
		}
		configureRemoteCaption(handle)
		d.win.OnClosed(cancel)
		if d.hidesTaskbar() {
			d.win.Show()
		}
		animationCtx, stopAnimation := context.WithCancel(ctx)
		go d.animate(animationCtx)
		go func() {
			defer stopAnimation()
			d.execute(ctx)
		}()
	})
	return mygo.App.Run()
}

func (d *remoteWindow) hidesTaskbar() bool {
	return runtime.GOOS == "windows" && d.action == "pull"
}

func (d *remoteWindow) windowOptions() mygo.WindowOptions {
	return mygo.WindowOptions{
		Title:             "MyGittu",
		TitleBarStyle:     mygo.TitleBarHidden,
		Hidden:            d.hidesTaskbar(),
		SkipTaskbar:       d.hidesTaskbar(),
		DisableMinimize:   true,
		DisableMaximize:   true,
		DisableFullScreen: true,
		HideWindowButtons: runtime.GOOS != "windows" && runtime.GOOS != "darwin",
		StateKey:          "remote",
		Width:             400, Height: 234, MinWidth: 360, MinHeight: 220,
		Content: native.View(d.view),
	}
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
	native.Box(c).Fill().Children(func() {
		d.content(c, t)
		if d.win != nil {
			bar := c.TitleBar()
			if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
				native.Box(c).Absolute().Top(0).Left(bar.Left).Right(bar.Right).Height(bar.Height).DragWindow()
			} else {
				native.Box(c).Absolute().Top(0).Left(0).Right(44).Height(34).DragWindow()
				label := d.text("关闭", "Close")
				if ui.IconButton(c, "", "close", false).Absolute().Top(4).Right(6).Label(label).Tooltip(label).Clicked() {
					d.win.Close()
				}
			}
		}
	})
}

func (d *remoteWindow) content(c *native.Context, t *native.Theme) {
	native.Column(c).Fill().Padding(8).Background(t.Surface).Children(func() {
		native.Column(c).Fill().MinHeight(0).Background(t.Background).Radius(8).Clip().Children(func() {
			d.cardContent(c, t)
		})
	})
}

func (d *remoteWindow) cardContent(c *native.Context, t *native.Theme) {
	if d.err == "" {
		message := d.status
		if message == "" {
			message = d.runningStatus()
		}
		native.Column(c).Fill().Background(t.Background).Padding(12).Center().Gap(8).Children(func() {
			if d.upToDate {
				native.Image(c, ui.UpToDateRabbitFrame(d.frame)).Size(116, 116)
			} else {
				native.Image(c, ui.RemoteRabbitFrame(d.action, d.frame)).Width(116).Height(116)
			}
			native.Text(c, message).FontSize(17 + t.FontSize - 12.5).TextAlign(native.Center).Bold()
		})
		return
	}
	native.Column(c).Fill().Padding(36, 18, 18, 18).Gap(12).Background(t.Background).Children(func() {
		native.Text(c, d.status).FontSize(17 + t.FontSize - 12.5).Bold().TextColor(t.Danger)
		native.Scroll(c).Grow(1).FillWidth().Background(t.Surface).Radius(8).Children(func() {
			native.Text(c, d.err).FillWidth().Padding(10)
		})
	})
}

func (d *remoteWindow) runningStatus() string {
	switch d.action {
	case "fetch":
		return d.text("正在获取远程更新…", "Fetching remote updates…")
	case "push":
		return d.text("正在检查推送状态…", "Checking push status…")
	default:
		return d.text("正在拉取远程更新…", "Pulling remote updates…")
	}
}

func (d *remoteWindow) animate(ctx context.Context) {
	ticker := time.NewTicker(ui.RemoteFrameInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mygo.RunOnMain(func() {
				if d.win.IsDestroyed() {
					return
				}
				if d.upToDate && d.frame < 3 {
					d.frame++
					d.win.Invalidate()
				} else if !d.finished {
					d.frame = (d.frame + 1) % ui.RemoteFrameCount
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
	var result app.RemoteResult
	if err := d.client.Call(ctx, "git.remote", ipc.Params{RepoID: repo.ID, Action: d.action}, &result); err != nil {
		d.fail(err)
		return
	}
	d.win.Update(func() {
		d.complete(result)
	})
	duration := 900 * time.Millisecond
	if result.NothingToPush || result.UpToDate {
		duration = 1800 * time.Millisecond
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		d.win.Close()
	case <-ctx.Done():
	}
}

func (d *remoteWindow) complete(result app.RemoteResult) {
	d.finished, d.frame = true, 3
	d.status = d.text("已完成", "Completed")
	d.upToDate = result.NothingToPush || result.UpToDate
	if d.upToDate {
		d.frame = 0
	}
	switch {
	case result.NothingToPush:
		d.status = d.text("当前无内容推送", "Nothing to push")
	case result.UpToDate && d.action == "pull":
		d.status = d.text("当前分支已是最新", "Current branch is up to date")
	case result.UpToDate:
		d.status = d.text("没有远程更新", "No remote updates")
	}
}

func (d *remoteWindow) fail(err error) {
	d.win.Update(func() {
		d.finished, d.frame = true, 0
		d.status = d.text("操作失败", "Operation failed")
		d.err = strings.TrimSpace(err.Error())
	})
}
