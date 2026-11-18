package ui

import (
	"context"
	native "github.com/egoist/mygo/ui"
	"mygittu/internal/buildinfo"
	"strings"
)

type integrationState struct {
	Startup         bool
	Menu            bool
	Modern          bool
	ModernAvailable bool
	Overlays        bool
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
				if state.Menu && strings.Contains(strings.ToUpper(d.integrationError), "0X800B0109") {
					d.integrationError = d.copy(
						"Windows 未信任新版菜单的 MSIX 签名证书（0x800B0109）。可通过「显示更多选项」使用 MyGittu 菜单。信任发布证书后可重试注册。",
						"Windows does not trust the modern menu’s MSIX signing certificate (0x800B0109). Use MyGittu under Show more options. Retry registration after trusting the publisher certificate.")
				}
			} else if statusErr != nil {
				d.integrationError = statusErr.Error()
			}
		}, nil
	})
}

func (d *Desktop) integrationSettings(c *native.Context) {
	t := c.Theme()
	native.Text(c, d.copy("当前版本", "Current version")+"  "+buildinfo.Version).TextColor(t.TextMuted)
	if IconButton(c, d.copy("克隆凭据", "Clone credentials"), "user", false).Disabled(d.busy).Clicked() {
		d.settingsTab = 6
	}

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
			if d.integrationError == "" {
				native.Text(c, d.copy("可通过「显示更多选项」使用 MyGittu 菜单。", "Use MyGittu under Show more options.")).FillWidth().TextColor(t.TextMuted)
			}
			if d.integration.ModernAvailable && IconButton(c, d.copy("重试新版菜单", "Retry modern menu"), "refresh", false).Disabled(d.busy).Clicked() {
				d.runIntegration("Modern", true)
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
