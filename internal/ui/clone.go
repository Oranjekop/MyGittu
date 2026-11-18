package ui

import (
	"context"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/egoist/mygo"
	native "github.com/egoist/mygo/ui"
	git "mygittu/internal/git"
	"mygittu/internal/ipc"
	"mygittu/internal/repository"
)

var cloneHosts = [...]string{"", "github.com", "gitee.com", "gitlab.com", "bitbucket.org"}
var repoPath = regexp.MustCompile(`^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)+$`)

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
	native.Text(c, d.text("clone.title")).Bold()
	native.Segmented(c, &d.cloneProvider, d.copy("Git 地址", "Git URL"), "GitHub", "Gitee", "GitLab", "Bitbucket").Disabled(d.busy)
	host := cloneHosts[d.cloneProvider]
	placeholder := d.text("clone.url")
	if host != "" {
		placeholder = d.copy("所有者/仓库 或仓库链接", "owner/repository or repository URL")
	}
	native.TextInput(c, &d.cloneURL).Placeholder(placeholder).FillWidth().Disabled(d.busy)
	lookupURL := providerCloneURL(host, d.cloneURL, false)
	profile := d.chooseCloneProfile(c, host, lookupURL)
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
		native.TextInput(c, &d.cloneDestination).Placeholder(d.text("clone.destination")).Grow(1).Disabled(d.busy)
		if IconButton(c, d.copy("选择位置", "Choose location"), "open", false).Disabled(d.busy).Clicked() {
			initial := d.path
			d.task(func(context.Context) (func(), error) {
				paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: d.win, Directory: true, Title: d.text("clone.destination"), DefaultPath: initial})
				return func() {
					if len(paths) == 0 {
						return
					}
					name := strings.TrimSuffix(filepath.Base(strings.ReplaceAll(remote, ":", "/")), ".git")
					if name == "" || name == "." || name == ".." || name == string(filepath.Separator) {
						name = "my-repository"
					}
					d.cloneDestination = filepath.Join(paths[0], name)
				}, err
			})
		}
	})
	destination := strings.TrimSpace(d.cloneDestination)
	if IconButton(c, d.text("clone.run"), "clone", true).Disabled(d.busy || !ready || remote == "" || !filepath.IsAbs(destination)).Clicked() {
		d.operationTask("clone.running", func(ctx context.Context) (func(), error) {
			var repo repository.Info
			err := d.Client.Call(ctx, "git.clone", ipc.Params{URL: remote, Destination: destination, CloneAuth: auth}, &repo)
			return func() {
				d.repos = append(d.repos, repo)
				d.selectRepository(repo.ID, repo.Root)
				d.page = 0
				d.refresh()
			}, err
		})
	}
}
