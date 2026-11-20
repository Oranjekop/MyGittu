package ui

import (
	"fmt"
	"runtime"
	"strings"

	native "github.com/egoist/mygo/ui"
	"mygittu/internal/buildinfo"
	"mygittu/internal/commitai"
	"mygittu/internal/settings"
)

const sourceRepositoryURL = "https://github.com/Oranjekop/MyGittu"

func (d *Desktop) openSettings() {
	d.settingsOpen = true
	d.settingsTab = 0
	d.resetPromptDraft()
	d.aiForm.apply(d.prefs.AI)
	d.aiForm.key = d.aiForm.savedKey
	d.cloneCredentials.clearEditor()
	if d.selected == "" {
		d.identityScope = 1
	}
}
func (d *Desktop) settingsDialog(c *native.Context) {
	width, height := c.Size()
	dialogWidth := min(float32(780), max(420, width-64))
	bodyHeight := min(float32(422), max(245, height-136))
	t := c.Theme()
	tabs := []struct{ label, icon string }{
		{d.copy("外观", "Appearance"), "appearance"},
		{d.text("identity.title"), "user"},
		{d.copy("AI 配置", "AI settings"), "sparkle"},
		{d.text("update.title"), "download"},
		{d.copy("系统集成", "Integration"), "settings"},
		{d.copy("克隆凭据", "Clone credentials"), "user"},
	}
	native.Modal(c, &d.settingsOpen, func() {
		native.Column(c).Width(dialogWidth).MaxWidth(dialogWidth).Gap(14).Children(func() {
			native.Row(c).FillWidth().Gap(controlSpacing).AlignItems(native.Center).Children(func() {
				native.Text(c, d.text("nav.settings")).FontSize(18).Bold().Grow(1)
				sourceLabel := d.copy("打开开源仓库", "Open source repository")
				if IconButton(c, "", "external", false).Label(sourceLabel).Tooltip(sourceRepositoryURL).Border(0, t.Border).Clicked() {
					c.OpenURL(sourceRepositoryURL)
				}
				if IconButton(c, "", "close", false).Label(d.text("close")).Tooltip(d.text("close")).Border(0, t.Border).Clicked() {
					d.settingsOpen = false
				}
			})
			native.Box(c).FillWidth().Height(1).Background(t.Border)
			native.Row(c).Height(bodyHeight).FillWidth().Gap(18).Children(func() {
				native.Column(c).Width(max(140, t.FontSize*10)).FillHeight().Padding(6).
					Gap(3).Background(t.Surface).Radius(6).Children(func() {
					for i, tab := range tabs {
						if IconButton(c, tab.label, tab.icon, d.settingsTab == i).FillWidth().Justify(native.Start).
							Border(0, t.Border).Clicked() && d.settingsTab != i {
							d.settingsTab = i
							if i == 4 && !d.busy {
								d.queryIntegration()
							}

							if i == 1 && d.Client != nil {
								d.loadIdentity()
							}
						}
					}
				})
				native.Column(c).Grow(1).MinWidth(0).FillHeight().Gap(12).Children(func() {
					native.Text(c, tabs[d.settingsTab].label).FontSize(max(16, t.FontSize)).Bold()
					if d.err != "" {
						native.Text(c, d.err).FillWidth().TextColor(t.Danger)
					}
					if d.settingsTab == 5 {
						d.cloneSettings(c)
						return
					}
					native.Scroll(c.Key(d.settingsTab)).PaddingEnd(scrollbarGutter(c)).Grow(1).MinHeight(0).FillWidth().Children(func() {
						native.Column(c).FillWidth().Gap(12).Children(func() {
							switch d.settingsTab {
							case 0:
								d.appearanceSettings(c)
							case 1:
								d.identitySettings(c)
							case 2:
								d.aiSettings(c)

								d.promptSettings(c)
							case 3:
								d.updateSettings(c)
							case 4:
								d.integrationSettings(c)
							}
						})
					})
				})
			})
		})
	})
}
func (d *Desktop) updateSettings(c *native.Context) {
	t := c.Theme()
	native.Row(c).FillWidth().Gap(panelSpacing).AlignItems(native.Center).Wrap().Children(func() {
		native.Text(c, d.copy("当前版本", "Current version")+"  "+buildinfo.Version).TextColor(t.TextMuted)
		if IconButton(c, d.text("update.check"), "refresh", false).Disabled(d.updateChecking).Clicked() {
			d.checkUpdate(true)
		}
		if d.updateChecking {
			native.Text(c, d.text("busy"))
		}
	})
	if d.update.Available {
		label := fmt.Sprintf(d.text("update.install"), d.update.Latest)
		if d.updateInstalling {
			label = d.text("update.downloading")
		}
		if IconButton(c, label, "download", false).Disabled(d.updateInstalling).Clicked() {
			d.installUpdate()
		}
	}
	if d.updateError != "" {
		native.Text(c, d.updateError)
	}
	if d.updateInstalling {
		if d.updateTotal > 0 {
			native.Progress(c, min(1, float64(d.updateBytes)/float64(d.updateTotal))).FillWidth()
		} else if d.updateBytes > 0 {
			native.Textf(c, "%.1f MiB", float64(d.updateBytes)/(1<<20)).TextColor(t.TextMuted)
		}
		if IconButton(c, d.text("update.cancel"), "close", false).Clicked() && d.updateCancel != nil {
			d.updateCancel()
		}
	}
	if !d.updateChecking && d.update.Latest != "" && !d.update.Available {
		native.Text(c, d.text("update.current"))
	}
}
func (d *Desktop) identitySettings(c *native.Context) {
	t := c.Theme()
	scopeControl := native.Segmented(c, &d.identityScope, d.text("identity.local"), d.text("identity.global")).Disabled(d.busy || d.selected == "")
	if scopeControl.Changed() && !d.busy && d.selected != "" && d.Client != nil {
		d.loadIdentity()
	}
	native.Row(c).Gap(panelSpacing).Children(func() {
		native.Column(c).Grow(1).Gap(controlSpacing).Children(func() {
			native.Text(c, d.text("identity.name"))
			native.TextInput(c, &d.identity.Name).FillWidth().Disabled(d.busy)
		})
		native.Column(c).Grow(1).Gap(controlSpacing).Children(func() {
			native.Text(c, d.text("identity.email"))
			native.TextInput(c, &d.identity.Email).FillWidth().Disabled(d.busy)
		})
	})
	native.Row(c).Gap(panelSpacing).Children(func() {
		if IconButton(c, d.text("identity.save"), "save", true).Disabled(d.busy || strings.TrimSpace(d.identity.Name) == "" || strings.TrimSpace(d.identity.Email) == "").Clicked() {
			d.saveIdentity()
		}
		if d.identitySaved {
			native.Text(c, d.text("identity.saved"))
		}
	})
	native.Text(c, d.text("identity.future")).TextColor(t.TextMuted)
}
func supportsHideOnClose() bool {
	return runtime.GOOS == "windows" || runtime.GOOS == "darwin"
}

func (d *Desktop) appearanceSettings(c *native.Context) {
	if supportsHideOnClose() {
		native.Row(c).FillWidth().Gap(controlSpacing).AlignItems(native.Center).Wrap().Children(func() {
			hide := d.prefs.MinimizeOnClose
			label := d.copy("关闭主窗口时隐藏到系统托盘", "Hide the main window to the system tray on close")
			if runtime.GOOS == "darwin" {
				label = d.copy("关闭主窗口时隐藏到菜单栏", "Hide the main window to the menu bar on close")
			}
			native.Checkbox(c, &hide, label).Disabled(d.busy)
			if hide != d.prefs.MinimizeOnClose {
				prefs := d.prefs
				prefs.MinimizeOnClose = hide
				d.savePrefs(prefs)
			}
		})
	}
	native.Text(c, d.text("settings.language"))
	oldLanguage := d.languageIndex
	native.Segmented(c, &d.languageIndex, "简体中文", "English").Disabled(d.busy)
	if oldLanguage != d.languageIndex {
		v := d.prefs
		v.Language = "zh-CN"
		if d.languageIndex == 1 {
			v.Language = "en"
		}
		d.savePrefs(v)
	}
	native.Text(c, d.text("settings.theme"))
	oldTheme := d.themeIndex
	native.Segmented(c, &d.themeIndex, d.text("theme.system"), d.text("theme.light"), d.text("theme.dark")).Disabled(d.busy)
	if oldTheme != d.themeIndex {
		v := d.prefs
		v.Theme = []string{"system", "light", "dark"}[d.themeIndex]
		d.savePrefs(v)
	}
	native.Text(c, d.copy("字体大小", "Font size")).Bold()
	native.Row(c).Gap(controlSpacing).AlignItems(native.Center).Children(func() {
		size := d.prefs.TextSize()
		if IconButton(c, "", "minus", false).Tooltip(d.copy("缩小字号", "Decrease font size")).Label(d.copy("缩小字号", "Decrease font size")).Disabled(d.busy || size <= 11).Clicked() {
			d.changeFontSize(max(11, size-1))
		}
		native.Textf(c, "%g", size).Width(32)
		if IconButton(c, "", "plus", false).Tooltip(d.copy("增大字号", "Increase font size")).Label(d.copy("增大字号", "Increase font size")).Disabled(d.busy || size >= 18).Clicked() {
			d.changeFontSize(min(18, size+1))
		}
		if IconButton(c, d.copy("恢复默认", "Reset"), "undo", false).Disabled(d.busy || size == (settings.Values{}).TextSize()).Clicked() {
			d.changeFontSize(0)
		}
	})

}

func (d *Desktop) changeFontSize(size float32) {
	prefs := d.prefs
	prefs.FontSize = size
	d.savePrefs(prefs)
}

func (d *Desktop) resetPromptDraft() {
	d.aiPromptDraft = d.prefs.AIPrompt
	if d.aiPromptDraft == "" {
		d.aiPromptDraft = commitai.DefaultPrompt()
	}
}
