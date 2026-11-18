package ui

import (
	"context"
	native "github.com/egoist/mygo/ui"
	"mygittu/internal/buildinfo"
)

type integrationState struct {
	Startup  bool
	Menu     bool
	Modern   bool
	Overlays bool
}

func (d *Desktop) queryIntegration() {
	d.integrationLoaded = false
	d.runIntegration("Status", false)
}

func (d *Desktop) runIntegration(action string, enabled bool) {
	d.integrationError = ""
	d.task(func(ctx context.Context) (func(), error) {
		var actionErr error
		if action != "Status" {
			actionErr = changeIntegration(ctx, action, enabled)
		}
		state, statusErr := readIntegration(ctx)
		return func() {
			d.integrationLoaded = statusErr == nil
			if statusErr == nil {
				d.integration = state
			}
			if actionErr != nil {
				d.integrationError = actionErr.Error()
			} else if statusErr != nil {
				d.integrationError = statusErr.Error()
			}
		}, nil
	})
}

func (d *Desktop) integrationSettings(c *native.Context) {
	t := c.Theme()
	native.Text(c, d.copy("当前版本", "Current version")+"  "+buildinfo.Version).TextColor(t.TextMuted)
	d.cloneSettings(c)

	if d.integrationError != "" {
		native.Text(c, d.integrationError).FillWidth().TextColor(t.Danger)
	}
	if !d.integrationLoaded {
		if IconButton(c, d.copy("读取配置", "Read configuration"), "refresh", false).Disabled(d.busy).Clicked() {
			d.queryIntegration()
		}
		return
	}
	d.integrationSwitch(c, d.copy("登录时启动 MyGittu", "Launch MyGittu at login"), "Startup", d.integration.Startup)
	if explorerSupported {
		d.integrationSwitch(c, d.copy("资源管理器右键菜单", "Explorer context menu"), "Menu", d.integration.Menu)
		d.integrationSwitch(c, d.copy("Git 状态徽标", "Git status badges"), "Overlays", d.integration.Overlays)
		if modernSupported() && d.integration.Menu && !d.integration.Modern {
			native.Text(c, d.copy("新版菜单尚未注册。若使用自签名组件，请先信任发布证书，再重试。", "The modern menu is not registered. If using the self-signed package, trust the publisher certificate and retry.")).FillWidth().TextColor(t.TextMuted)
			if IconButton(c, d.copy("重试新版菜单", "Retry modern menu"), "refresh", false).Disabled(d.busy).Clicked() {
				d.runIntegration("Menu", true)
			}
		}
		if IconButton(c, d.copy("重启资源管理器以应用", "Restart Explorer to apply"), "refresh", false).Disabled(d.busy).Clicked() {
			d.runIntegration("Refresh", false)
		}
	}
}

func (d *Desktop) integrationSwitch(c *native.Context, label, action string, current bool) {
	next := current
	native.Checkbox(c, &next, label).Disabled(d.busy)
	if next != current {
		d.runIntegration(action, next)
	}
}
