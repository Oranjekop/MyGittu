package ui

import (
	"context"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/egoist/mygo"
	native "github.com/egoist/mygo/ui"
	git "mygittu/internal/git"
	"mygittu/internal/ipc"
	"mygittu/internal/repository"
)

var cloneHosts = [...]string{"", "github.com", "gitee.com", "gitlab.com", "bitbucket.org"}
var repoPath = regexp.MustCompile(`^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)+$`)

const cloneOperation = "clone.running"

func providerCloneURL(host, input string, ssh bool) string {
	input = strings.TrimSpace(input)
	if host == "" {
		return input
	}
	if rest, ok := strings.CutPrefix(input, "git@"+host+":"); ok {
		input = rest
	}
	if u, err := url.Parse(input); err == nil && u.IsAbs() {
		if (u.Scheme != "https" && u.Scheme != "ssh") || !strings.EqualFold(u.Host, host) || u.RawQuery != "" || u.Fragment != "" || (u.User != nil && (u.Scheme != "ssh" || u.User.String() != "git")) {
			return ""
		}
		input = u.Path
	}
	input = strings.TrimSuffix(strings.Trim(input, "/"), ".git")
	if !repoPath.MatchString(input) {
		return ""
	}
	for _, part := range strings.Split(input, "/") {
		if part == "." || part == ".." {
			return ""
		}
	}
	if ssh {
		return "git@" + host + ":" + input + ".git"
	}
	return "https://" + host + "/" + input + ".git"
}

func (d *Desktop) cloneView(c *native.Context) {
	if d.operation == cloneOperation {
		d.cloneProgress(c)
		return
	}
	native.Column(c).FillWidth().MaxWidth(760).Padding(panelSpacing).Gap(panelSpacing).
		Background(c.Theme().Background).Radius(8).Children(func() { d.cloneForm(c) })
}

func (d *Desktop) cloneDialog(c *native.Context) {
	if !d.cloneOpen || d.settingsOpen {
		return
	}
	width, height := c.Size()
	running := d.operation == cloneOperation
	dismissed := native.Modal(c, &d.cloneOpen, func() {
		native.Column(c).Width(min(670, width-80)).Gap(panelSpacing).Children(func() {
			native.Row(c).FillWidth().Gap(controlSpacing).AlignItems(native.Center).Children(func() {
				native.Text(c, d.text("clone.title")).FontSize(18).Bold().Grow(1)
				if IconButton(c, d.text("close"), "close", false).Disabled(running).Clicked() {
					d.cloneOpen = false
					d.err = ""
				}
			})
			if running {
				native.Column(c).Height(220).FillWidth().Children(func() { d.cloneProgress(c) })
				return
			}
			native.Scroll(c).PaddingEnd(scrollbarGutter(c)).FillWidth().MaxHeight(height - 130).Children(func() {
				native.Column(c).FillWidth().Gap(panelSpacing).Children(func() {
					if d.err != "" {
						native.Text(c, d.err).FillWidth().TextColor(c.Theme().Danger)
					}
					d.cloneForm(c)
				})
			})
		})
	}).Dismissed()
	if dismissed {
		d.cloneOpen = running
		if !running {
			d.err = ""
		}
	}
}

func (d *Desktop) cloneProgress(c *native.Context) {
	t := c.Theme()
	native.Column(c).Grow(1).FillWidth().Padding(20).Gap(controlSpacing).Center().
		Background(t.Background).Radius(8).Children(func() {
		native.Image(c, RemoteRabbitFrame("pull", d.operationFrame)).Size(116, 116)
		native.Text(c, d.text(cloneOperation)).FontSize(t.FontSize + 4).Bold()
		if destination := strings.TrimSpace(d.cloneDestination); destination != "" {
			native.Text(c, destination).SingleLine().FillWidth().MaxWidth(640).TextAlign(native.Center).
				Tooltip(destination).TextColor(t.TextMuted)
		}
	})
}

func (d *Desktop) cloneForm(c *native.Context) {
	native.Segmented(c, &d.cloneProvider, d.copy("Git 地址", "Git URL"), "GitHub", "Gitee", "GitLab", "Bitbucket").Disabled(d.busy)
	host := cloneHosts[d.cloneProvider]
	placeholder := d.text("clone.url")
	if host != "" {
		placeholder = d.copy("所有者/仓库 或仓库链接", "owner/repository or repository URL")
	}
	if host != "github.com" {
		d.cloneAddressInput(c, placeholder)
	}
	lookupURL := providerCloneURL(host, d.cloneURL, false)
	profile := d.chooseCloneProfile(c, host, lookupURL)
	if host == "github.com" {
		d.githubRepositoryPicker(c, profile.Username, profile.Kind == "github")
		d.cloneAddressInput(c, placeholder)
	}
	remote := providerCloneURL(host, d.cloneURL, profile.Kind == "ssh")
	var auth git.CloneAuth
	ready := true
	switch profile.Kind {
	case "https":
		auth.Username, auth.Password = profile.Username, d.cloneCredentials.secrets[profile.ID]
		ready = auth.Password != "" && strings.HasPrefix(remote, "https://")
		if auth.Password == "" {
			native.Text(c, d.copy("请在设置中为此凭据输入密码或令牌。", "Enter the password or token for this credential in Settings.")).TextColor(c.Theme().TextMuted)
		}
	case "ssh":
		auth.SSHKey = profile.SSHKey
	case "github":
		auth.GitHubAccount = profile.Username
	}
	native.Row(c).Gap(controlSpacing).Children(func() {
		if d.cloneParent != "" {
			name := strings.TrimSuffix(filepath.Base(strings.ReplaceAll(remote, ":", "/")), ".git")
			if remote == "" || name == "." || name == ".." || name == "" || name == string(filepath.Separator) {
				name = "my-repository"
			}
			d.cloneDestination = filepath.Join(d.cloneParent, name)
		}
		before := d.cloneDestination
		native.TextInput(c, &d.cloneDestination).Placeholder(d.text("clone.destination")).Grow(1).Disabled(d.busy)
		if before != d.cloneDestination {
			d.cloneParent = ""
		}
		if IconButton(c, d.copy("选择位置", "Choose location"), "open", false).Disabled(d.busy).Clicked() {
			initial := d.cloneParent
			if initial == "" {
				initial = d.path
			}
			d.task(func(context.Context) (func(), error) {
				paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: d.win, Directory: true, Title: d.text("clone.destination"), DefaultPath: initial})
				return func() {
					if len(paths) == 0 {
						return
					}
					d.cloneParent = paths[0]
				}, err
			})
		}
	})
	destination := strings.TrimSpace(d.cloneDestination)
	if displayURL := git.Redact(remote); remote != "" && displayURL != strings.TrimSpace(d.cloneURL) {
		native.Text(c, displayURL).FillWidth().TextColor(c.Theme().TextMuted)
	}
	if strings.TrimSpace(d.cloneURL) != "" && remote == "" {
		native.Text(c, d.copy("请输入有效的所有者/仓库或仓库链接。", "Enter a valid owner/repository or repository URL.")).TextColor(c.Theme().Danger)
	}
	if destination != "" && !filepath.IsAbs(destination) {
		native.Text(c, d.copy("请输入绝对路径或选择本地文件夹。", "Enter an absolute path or choose a local folder.")).TextColor(c.Theme().Danger)
	}
	if IconButton(c, d.text("clone.run"), "clone", true).Disabled(d.busy || !ready || remote == "" || !filepath.IsAbs(destination)).Clicked() {
		d.operationTask(cloneOperation, func(ctx context.Context) (func(), error) {
			var repo repository.Info
			err := d.Client.Call(ctx, "git.clone", ipc.Params{URL: remote, Destination: destination, CloneAuth: auth}, &repo)
			return func() {
				if d.action == "clone" {
					d.path = repo.Root
					d.result = d.copy("克隆成功", "Clone completed")
					return
				}
				d.repos = append(d.repos, repo)
				d.cloneOpen = false
				d.selectRepository(repo.ID, repo.Root)
				d.page = 0
				d.refresh()
			}, err
		})
	}
}

func (d *Desktop) cloneAddressInput(c *native.Context, placeholder string) {
	before := d.cloneURL
	native.TextInput(c, &d.cloneURL).Placeholder(placeholder).FillWidth().Disabled(d.busy)
	if before != d.cloneURL {
		d.cloneRepoChoice = ""
	}
}

func (d *Desktop) githubRepositoryPicker(c *native.Context, account string, enabled bool) {
	if !enabled {
		native.Text(c, d.copy("选择 GitHub 登录凭据后可浏览仓库。", "Select a GitHub login credential to browse repositories.")).TextColor(c.Theme().TextMuted)
		return
	}
	load := func() {
		d.cloneAccount, d.cloneRepoSearch, d.cloneRepoChoice = account, "", ""
		d.cloneRepos = nil
		d.task(func(ctx context.Context) (func(), error) {
			var names []string
			err := d.Client.Call(ctx, "git.github.repositories", ipc.Params{Name: account}, &names)
			slices.Sort(names)
			return func() { d.cloneRepos = append([]string{}, names...) }, err
		})
	}
	if d.cloneAccount != account && d.Client != nil && !d.busy {
		load()
	}
	native.Row(c).Gap(controlSpacing).Children(func() {
		native.TextInput(c, &d.cloneRepoSearch).Placeholder(d.copy("筛选仓库", "Filter repositories")).Grow(1).Disabled(d.busy)
		if IconButton(c, d.copy("刷新 GitHub 仓库", "Refresh GitHub repositories"), "refresh", false).Disabled(d.busy || d.Client == nil).Clicked() {
			load()
		}
	})
	if d.cloneAccount != account || d.cloneRepos == nil {
		if d.busy {
			native.Text(c, d.copy("正在读取 GitHub 仓库…", "Loading GitHub repositories…")).TextColor(c.Theme().TextMuted)
		}
		return
	}
	var matches []string
	query := strings.ToLower(strings.TrimSpace(d.cloneRepoSearch))
	for _, name := range d.cloneRepos {
		if strings.Contains(strings.ToLower(name), query) {
			matches = append(matches, name)
		}
	}
	if len(matches) == 0 {
		if !d.busy && d.err == "" {
			emptyState(c, emptyPreviewArt, 64, d.copy("没有匹配的仓库。", "No matching repositories."))
		}
		return
	}
	label := d.copy("选择 GitHub 仓库", "Select a GitHub repository")
	choice := d.cloneRepoChoice
	if choice == "" {
		choice = label
	}
	native.Select(c, &choice, matches).Label(label).FillWidth().Disabled(d.busy)
	if choice != label && choice != d.cloneRepoChoice {
		d.cloneRepoChoice, d.cloneURL = choice, choice
	}
}
