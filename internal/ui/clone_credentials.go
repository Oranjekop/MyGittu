package ui

import (
	"context"
	"crypto/rand"
	"net/url"
	"slices"
	"strings"

	"github.com/egoist/mygo"
	native "github.com/egoist/mygo/ui"
	"mygittu/internal/ipc"
	"mygittu/internal/settings"
)

type cloneCredentials struct {
	draft       settings.CloneProfile
	password    string
	selected    string
	secrets     map[string]string
	authorized  string
	authorizing bool
	list        native.ListState
	deleteID    string
}

func (f *cloneCredentials) clearEditor() {
	f.draft = settings.CloneProfile{Kind: "https"}
	f.password, f.authorized, f.deleteID = "", "", ""
}

func (f *cloneCredentials) edit(profiles []settings.CloneProfile, id string) {
	f.clearEditor()
	for _, p := range profiles {
		if p.ID == id {
			f.draft = p
			f.password = f.secrets[p.ID]
			return
		}
	}
}

func (d *Desktop) cloneSettings(c *native.Context) {
	f := &d.cloneCredentials
	if f.draft.Kind == "" {
		f.clearEditor()
	}
	native.Row(c).Grow(1).MinHeight(0).FillWidth().Gap(14).Children(func() {
		native.Column(c).WidthPercent(39).FillHeight().Gap(controlSpacing).Children(func() {
			d.cloneCredentialList(c, f)
		})
		native.Box(c).Width(1).FillHeight().Background(c.Theme().Border)
		scrollAtPanelEdge(native.Scroll(c)).Grow(1).MinWidth(0).FillHeight().Children(func() {
			native.Column(c).FillWidth().PaddingEnd(detailSpacing).Gap(controlSpacing).Children(func() {
				d.cloneCredentialEditor(c, f)
			})
		})
	})
}

func (d *Desktop) cloneCredentialList(c *native.Context, f *cloneCredentials) {
	native.Row(c).FillWidth().AlignItems(native.Center).Gap(detailSpacing).Wrap().Children(func() {
		if IconButton(c, d.copy("新增凭据", "Add credential"), "plus", false).Disabled(d.busy).Clicked() {
			f.clearEditor()
		}
		if f.draft.ID != "" {
			label := d.copy("删除凭据", "Delete credential")
			if IconButton(c, "", "remove", false).Label(label).Tooltip(label).Disabled(d.busy).Clicked() {
				f.deleteID = f.draft.ID
			}
		}
	})
	profiles := d.prefs.CloneProfiles
	if len(profiles) > 0 {
		row := slices.IndexFunc(profiles, func(p settings.CloneProfile) bool { return p.ID == f.draft.ID })
		before := row
		f.list.Selected = &row
		roundedTable(c, &f.list, []native.TableColumn{
			{Title: d.copy("名称", "Name")}, {Title: d.copy("类型", "Type"), Width: 68},
		}, len(profiles), func(row, col int) {
			p := profiles[row]
			if col == 0 {
				native.Text(c, p.Label).SingleLine().Ellipsis("…")
				return
			}
			kind := strings.ToUpper(p.Kind)
			if p.Kind == "github" {
				kind = "GitHub"
			}
			native.Text(c, kind)
		}).Grow(1).MinHeight(0).Disabled(d.busy)
		if row != before && row >= 0 && row < len(profiles) {
			f.edit(profiles, profiles[row].ID)
		}
	} else if !d.busy && d.err == "" {
		emptyState(c, emptyPreviewArt, 72, d.copy("暂无凭据", "No saved credentials")).Grow(1)
	}
}

func (d *Desktop) cloneCredentialEditor(c *native.Context, f *cloneCredentials) {
	if f.deleteID != "" {
		native.Row(c).Gap(controlSpacing).Wrap().Children(func() {
			native.Text(c, d.copy("删除此凭据配置？", "Delete this credential profile?"))
			if IconButton(c, d.copy("确认删除", "Confirm deletion"), "remove", true).Disabled(d.busy).Clicked() {
				d.saveCloneProfile(f.draft, true)
			}
			if IconButton(c, d.copy("取消", "Cancel"), "close", false).Disabled(d.busy).Clicked() {
				f.deleteID = ""
			}
		})
		return
	}
	title := d.copy("新建凭据", "New credential")
	if f.draft.ID != "" {
		title = d.copy("编辑凭据", "Edit credential")
	}
	native.Text(c, title).Bold()
	native.Text(c, d.copy("显示名称", "Display name"))
	native.TextInput(c, &f.draft.Label).Label(d.copy("显示名称", "Display name")).FillWidth().Disabled(d.busy)
	kinds := []string{"https", "ssh", "github"}
	kind := max(0, slices.Index(kinds, f.draft.Kind))
	native.Segmented(c, &kind, "HTTPS", "SSH", d.copy("GitHub 登录", "GitHub login")).Disabled(d.busy)
	if f.draft.Kind != kinds[kind] {
		f.draft = settings.CloneProfile{ID: f.draft.ID, Label: f.draft.Label, Kind: kinds[kind]}
		f.password, f.authorized, f.deleteID = "", "", ""
	}
	switch f.draft.Kind {
	case "https":
		native.Text(c, d.copy("服务器域名", "Server hostname"))
		native.TextInput(c, &f.draft.Host).Placeholder(d.copy("服务器域名，例如 gitee.com", "Server hostname, e.g. gitee.com")).FillWidth().Disabled(d.busy)
		native.Text(c, d.copy("HTTPS 用户名", "HTTPS username"))
		native.TextInput(c, &f.draft.Username).Label(d.copy("HTTPS 用户名", "HTTPS username")).FillWidth().Disabled(d.busy)
		native.TextInput(c, &f.password).Password().Placeholder(d.copy("密码或令牌（本次运行）", "Password or token (this session)")).FillWidth().Disabled(d.busy)
	case "ssh":
		native.Text(c, d.copy("SSH 私钥路径", "SSH private key path"))
		native.Row(c).Gap(controlSpacing).Children(func() {
			native.TextInput(c, &f.draft.SSHKey).Placeholder(d.copy("SSH 私钥路径", "SSH private key path")).Grow(1).Disabled(d.busy)
			if IconButton(c, d.copy("选择私钥", "Choose key"), "open", false).Disabled(d.busy).Clicked() {
				title := d.copy("选择 SSH 私钥", "Choose SSH private key")
				d.task(func(context.Context) (func(), error) {
					paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: d.win, Title: title})
					return func() {
						if len(paths) > 0 {
							f.draft.SSHKey = paths[0]
						}
					}, err
				})
			}
		})
	case "github":
		native.Text(c, d.copy("GitHub 用户名", "GitHub username"))
		native.TextInput(c, &f.draft.Username).Label(d.copy("GitHub 用户名", "GitHub username")).FillWidth().Disabled(d.busy)
		name := strings.TrimSpace(f.draft.Username)
		label := d.copy("在浏览器授权", "Authorize in browser")
		if f.authorizing {
			label = d.copy("等待浏览器授权…", "Waiting for browser authorization…")
		}
		if IconButton(c, label, "user", false).Disabled(d.busy || name == "").Clicked() {
			d.authorizeCloneAccount(name)
		}
		if strings.EqualFold(name, f.authorized) && f.authorized != "" {
			native.Text(c, d.copy("授权完成。保存此凭据后可在克隆页选择 GitHub 仓库。", "Authorization complete. Save this credential to select GitHub repositories in Clone.")).FillWidth().TextColor(c.Theme().TextMuted)
		}
	}
	p := f.draft
	p.Label, p.Host, p.Username, p.SSHKey = strings.TrimSpace(p.Label), strings.ToLower(strings.TrimSpace(p.Host)), strings.TrimSpace(p.Username), strings.TrimSpace(p.SSHKey)
	if p.ID == "" {
		p.ID = "new"
	}
	native.Row(c).Gap(controlSpacing).Children(func() {
		if IconButton(c, d.copy("保存凭据", "Save credential"), "save", true).Disabled(d.busy || settings.ValidateCloneProfile(p) != nil).Clicked() {
			if f.draft.ID == "" {
				p.ID = rand.Text()
			}
			d.saveCloneProfile(p, false)
		}

	})
}

func (d *Desktop) authorizeCloneAccount(name string) {
	if d.busy {
		return
	}
	d.cloneCredentials.authorizing = true
	d.cloneCredentials.authorized = ""
	d.task(func(ctx context.Context) (func(), error) {
		err := d.Client.Call(ctx, "git.github.authorize", ipc.Params{Name: name}, nil)
		return func() {
			d.cloneCredentials.authorizing = false
			if err != nil {
				d.err = err.Error()
				return
			}
			d.cloneCredentials.authorized = name
			d.cloneAccount = ""
		}, nil
	})
}

func (d *Desktop) saveCloneProfile(p settings.CloneProfile, remove bool) {
	prefs := d.prefs
	prefs.CloneProfiles = slices.Clone(prefs.CloneProfiles)
	index := slices.IndexFunc(prefs.CloneProfiles, func(item settings.CloneProfile) bool { return item.ID == p.ID })
	switch {
	case remove && index >= 0:
		prefs.CloneProfiles = slices.Delete(prefs.CloneProfiles, index, index+1)
	case remove:
		return
	case index >= 0:
		prefs.CloneProfiles[index] = p
	default:
		prefs.CloneProfiles = append(prefs.CloneProfiles, p)
	}
	secret := d.cloneCredentials.password
	if index >= 0 && p.Kind == "https" {
		current := d.prefs.CloneProfiles[index]
		if (current.Host != p.Host || current.Username != p.Username) && secret == d.cloneCredentials.secrets[p.ID] {
			secret = ""
		}
	}
	d.task(func(ctx context.Context) (func(), error) {
		err := d.Client.Call(ctx, "settings.update", ipc.Params{Settings: prefs}, &prefs)
		return func() {
			d.applyPrefs(prefs)
			f := &d.cloneCredentials
			if remove {
				delete(f.secrets, p.ID)
				f.clearEditor()
				if f.selected == p.ID {
					f.selected = ""
				}
				return
			}
			if f.secrets == nil {
				f.secrets = make(map[string]string)
			}
			if p.Kind == "https" && secret != "" {
				f.secrets[p.ID] = secret
			} else {
				delete(f.secrets, p.ID)
			}
			f.draft, f.password = p, secret
			f.selected = p.ID
		}, err
	})
}

func cloneProfileMatches(p settings.CloneProfile, host string) bool {
	if p.Kind == "github" {
		return host == "github.com"
	}
	return p.Kind == "ssh" || (p.Kind == "https" && (p.Host == "" || strings.EqualFold(host, p.Host)))
}

func (d *Desktop) chooseCloneProfile(c *native.Context, host, remote string) settings.CloneProfile {
	if host == "" {
		if parsed, err := url.Parse(remote); err == nil {
			host = strings.ToLower(parsed.Hostname())
		}
	}
	f := &d.cloneCredentials
	choice := f.selected
	found := slices.ContainsFunc(d.prefs.CloneProfiles, func(p settings.CloneProfile) bool { return p.ID == choice && cloneProfileMatches(p, host) })
	if !found {
		choice = ""
	}
	native.Row(c).Gap(controlSpacing).Children(func() {
		var available []settings.CloneProfile
		for _, p := range d.prefs.CloneProfiles {
			if cloneProfileMatches(p, host) {
				available = append(available, p)
			}
		}
		d.cloneProfileMenu(c, &choice, d.copy("Git 默认凭据", "Git default credentials"), available).Grow(1)
		if IconButton(c, d.copy("配置克隆凭据", "Configure clone credentials"), "settings", false).Disabled(d.busy).Clicked() {
			d.openSettings()
			d.settingsTab = 5
			f.edit(d.prefs.CloneProfiles, choice)
			if host == "github.com" && choice == "" {
				f.draft.Kind, f.draft.Label = "github", "GitHub"
			}
		}
	})
	if found || choice != "" {
		f.selected = choice
	}
	for _, p := range d.prefs.CloneProfiles {
		if p.ID == choice && cloneProfileMatches(p, host) {
			return p
		}
	}
	return settings.CloneProfile{}
}

func (d *Desktop) cloneProfileMenu(c *native.Context, selected *string, empty string, profiles []settings.CloneProfile) native.Element {
	label := empty
	for _, p := range profiles {
		if p.ID == *selected {
			label = p.Label
			break
		}
	}
	return native.PopoverMenuButton(c, label, func(menu *native.Menu) {
		if menu.Item(empty).Radio(*selected == "").Chosen() {
			*selected = ""
		}
		for _, p := range profiles {
			if menu.Item(p.Label).Radio(*selected == p.ID).Chosen() {
				*selected = p.ID
			}
		}
	}, native.PopoverMenuButtonOptions{}).Disabled(d.busy)
}
