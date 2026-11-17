package ui

import (
	native "github.com/egoist/mygo/ui"
	"strings"
)

func (d *Desktop) openSettings() {
	d.settingsOpen = true
	if d.selected == "" {
		d.identityScope = 1
	}
	d.loadIdentity()
}
func (d *Desktop) settingsDialog(c *native.Context) {
	native.Modal(c, &d.settingsOpen, func() {
		native.Column(c).Width(540).MaxWidth(540).Gap(12).Children(func() {
			native.Row(c).Gap(8).AlignItems(native.Center).Children(func() {
				native.Icon(c, icons["settings"]).Width(20).Height(20)
				native.Text(c, d.text("nav.settings")).FontSize(20).Bold().Grow(1)
				if native.Button(c, d.text("close")).Clicked() {
					d.settingsOpen = false
				}
			})
			if d.err != "" {
				native.Text(c, d.err).TextColor(native.Hex("#bf3030"))
			}
			d.settingsContents(c)
		})
	})
}
func (d *Desktop) settingsContents(c *native.Context) {
	t := c.Theme()
	native.Row(c).Gap(10).Children(func() {
		native.Text(c, d.text("update.title")).Bold()
		if native.Button(c, d.text("update.check")).Disabled(d.updateChecking).Clicked() {
			d.checkUpdate(true)
		}
		if d.updateChecking {
			native.Text(c, d.text("busy"))
		}
	})
	if d.updateError != "" {
		native.Text(c, d.updateError)
	}
	if !d.updateChecking && d.update.Latest != "" && !d.update.Available {
		native.Text(c, d.text("update.current"))
	}
	native.Text(c, d.text("identity.title")).Bold()
	oldScope := d.identityScope
	native.Segmented(c, &d.identityScope, d.text("identity.local"), d.text("identity.global")).Disabled(d.busy || d.selected == "")
	if oldScope != d.identityScope {
		d.loadIdentity()
	}
	native.Row(c).Gap(12).Children(func() {
		native.Column(c).Grow(1).Gap(6).Children(func() {
			native.Text(c, d.text("identity.name"))
			native.TextInput(c, &d.identity.Name).FillWidth().Disabled(d.busy)
		})
		native.Column(c).Grow(1).Gap(6).Children(func() {
			native.Text(c, d.text("identity.email"))
			native.TextInput(c, &d.identity.Email).FillWidth().Disabled(d.busy)
		})
	})
	native.Row(c).Gap(12).Children(func() {
		if native.PrimaryButton(c, d.text("identity.save")).Disabled(d.busy || strings.TrimSpace(d.identity.Name) == "" || strings.TrimSpace(d.identity.Email) == "").Clicked() {
			d.saveIdentity()
		}
		if d.identitySaved {
			native.Text(c, d.text("identity.saved"))
		}
	})
	native.Text(c, d.text("identity.future")).TextColor(t.TextMuted)
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
	native.Text(c, d.text("settings.integration"))

}
