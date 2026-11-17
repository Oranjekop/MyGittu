package ui

import (
	"context"
	"fmt"
	"github.com/egoist/mygo"
	native "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"mygittu/internal/app"
	"mygittu/internal/brand"
	git "mygittu/internal/git"
	"mygittu/internal/i18n"
	"mygittu/internal/ipc"
	"mygittu/internal/repository"
	"mygittu/internal/settings"
	"mygittu/internal/status"
	"path/filepath"
	"strings"
	"time"
)

type Desktop struct {
	Client                             *ipc.Client
	win                                *mygo.Window
	repos                              []repository.Info
	selected                           string
	path, message, branch, detail, err string
	page                               int
	busy                               bool
	snapshot                           status.Snapshot
	logs                               []app.Commit
	row                                int
	files, history                     native.ListState
	staged                             bool
	offset                             int
	query                              string
	cloneURL, cloneDestination         string
	logRow                             int
	changes                            []git.FileStatus
	stagedCount                        int
	prefs                              settings.Values
	languageIndex, themeIndex          int
	showPath                           bool
	identity                           app.Identity
	identityScope                      int
	identitySaved                      bool
	update                             app.UpdateInfo
	updateChecking                     bool
	updateError                        string
	comparison                         *editor.DiffState
	binary                             bool
}

func New(client *ipc.Client, path, action string) *Desktop {
	d := &Desktop{Client: client, path: path, row: -1, logRow: -1, prefs: settings.Values{Language: "zh-CN", Theme: "system"}}
	switch action {
	case "log":
		d.page = 1
	case "diff":
		d.page = 2
	case "clone":
		d.page = 5
		if path != "" {
			d.cloneDestination = filepath.Join(path, "my-repository")
		}
	case "pull", "push":
		d.page = 6
		d.branch = action
	case "status", "commit", "":
	default:
		d.err = d.text("unsupported")
	}
	return d
}
func (d *Desktop) Run() error {
	mygo.App.WhenReady(func() {
		d.win = mygo.NewWindow(mygo.WindowOptions{Title: "MyGittu", Width: 1280, Height: 820, Content: native.View(d.view)})
		_ = d.win.SetIcon(brand.PNG)
		_ = mygo.App.Dock.SetIcon(brand.PNG)
		updateContext, cancelUpdates := context.WithCancel(context.Background())
		d.win.OnClosed(cancelUpdates)
		go d.watchUpdates(updateContext)
		d.win.OnFileDrop(func(e *mygo.FileDropEvent) {
			if len(e.Paths) > 0 {
				d.open(e.Paths[0])
			}
		})
		d.task(func(ctx context.Context) (func(), error) {
			var prefs settings.Values
			if e := d.Client.Call(ctx, "settings.get", ipc.Params{}, &prefs); e != nil {
				return nil, e
			}
			var repos []repository.Info
			e := d.Client.Call(ctx, "repo.list", ipc.Params{}, &repos)
			if e != nil {
				return nil, e
			}
			if d.path != "" && d.page != 5 {
				var repo repository.Info
				e = d.Client.Call(ctx, "repo.register", ipc.Params{Path: d.path}, &repo)
				if e != nil {
					return nil, e
				}
				e = d.Client.Call(ctx, "repo.list", ipc.Params{}, &repos)
				return func() { d.applyPrefs(prefs); d.repos = repos; d.selected = repo.ID; d.refresh() }, e
			}
			return func() {
				d.applyPrefs(prefs)
				d.repos = repos
				if len(repos) > 0 {
					d.selected = repos[0].ID
					d.refresh()
				}
			}, nil
		})
	})
	return mygo.App.Run()
}
func (d *Desktop) text(id string) string { return i18n.Text(d.prefs.Language, id) }
func (d *Desktop) applyPrefs(v settings.Values) {
	wasEmptyDiff := d.detail == d.text("diff.empty")
	d.prefs = v
	if wasEmptyDiff {
		d.detail = d.text("diff.empty")
	}
	d.languageIndex = 0
	if v.Language == "en" {
		d.languageIndex = 1
	}
	d.themeIndex = 0
	if v.Theme == "light" {
		d.themeIndex = 1
	}
	if v.Theme == "dark" {
		d.themeIndex = 2
	}
}
func (d *Desktop) savePrefs(v settings.Values) {
	d.task(func(ctx context.Context) (func(), error) {
		var prefs settings.Values
		e := d.Client.Call(ctx, "settings.update", ipc.Params{Settings: v}, &prefs)
		return func() { d.applyPrefs(prefs) }, e
	})
}

// All state is owned by the main thread. Background workers deliver one result.
func (d *Desktop) task(work func(context.Context) (func(), error)) {
	if d.busy {
		return
	}
	d.busy = true
	d.err = ""
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		apply, e := work(ctx)
		mygo.RunOnMain(func() {
			d.busy = false
			if e != nil {
				d.err = e.Error()
				d.applyPrefs(d.prefs)
			} else if apply != nil {
				apply()
			}
			if d.win != nil {
				d.win.Invalidate()
			}
		})
	}()
}
func (d *Desktop) refresh() {
	id, page, query, offset := d.selected, d.page, d.query, d.offset
	if id == "" && page != 4 {
		return
	}
	d.row = -1
	d.detail = ""
	d.comparison = nil
	d.task(func(ctx context.Context) (func(), error) {
		var snap status.Snapshot
		var logs []app.Commit
		var detail string
		var e error
		var prefs settings.Values
		var identity app.Identity
		switch page {
		case 0, 2:
			e = d.Client.Call(ctx, "status.refresh", ipc.Params{RepoID: id}, &snap)
			if e == nil && page == 0 {
				e = d.Client.Call(ctx, "identity.get", ipc.Params{RepoID: id, Scope: "local"}, &identity)
			}
		case 1:
			e = d.Client.Call(ctx, "git.log", ipc.Params{RepoID: id, Limit: 100, Offset: offset, Query: query}, &logs)
		case 3:
			e = d.Client.Call(ctx, "git.branches", ipc.Params{RepoID: id}, &detail)
		case 4:
			e = d.Client.Call(ctx, "settings.get", ipc.Params{}, &prefs)
			if e == nil {
				scope := "local"
				if d.identityScope == 1 || id == "" {
					scope = "global"
				}
				e = d.Client.Call(ctx, "identity.get", ipc.Params{RepoID: id, Scope: scope}, &identity)
			}
		}
		changes := make([]git.FileStatus, 0, len(snap.Files))
		stagedCount := 0
		for _, f := range snap.Files {
			if f.State != git.Ignored {
				changes = append(changes, f)
				if f.Index != git.Clean {
					stagedCount++
				}
			}
		}
		return func() {
			if page == 4 {
				d.applyPrefs(prefs)
				d.identity = editableIdentity(identity)
				if id == "" {
					d.identityScope = 1
				}
				d.identitySaved = false
			}
			if page == 0 {
				d.identity = editableIdentity(identity)
			}
			d.snapshot = snap
			d.changes = changes
			d.stagedCount = stagedCount
			d.logs = logs
			d.detail = detail
			d.logRow = -1
		}, e
	})
}
func (d *Desktop) mutate(method string, p ipc.Params) {
	p.RepoID = d.selected
	d.task(func(ctx context.Context) (func(), error) {
		e := d.Client.Call(ctx, method, p, nil)
		return func() {
			if method == "git.commit" {
				d.message = ""
			}
			d.refresh()
		}, e
	})
}
func (d *Desktop) visible() []git.FileStatus {
	return d.changes
}
func (d *Desktop) diff(path string) {
	id, staged := d.selected, d.staged
	original := ""
	for _, f := range d.changes {
		if f.Path == path {
			original = f.OriginalPath
			break
		}
	}
	d.task(func(ctx context.Context) (func(), error) {
		var comparison app.Comparison
		e := d.Client.Call(ctx, "git.comparison", ipc.Params{RepoID: id, Path: path, OriginalPath: original, Staged: staged}, &comparison)
		return func() {
			old, next := editor.NewBuffer(comparison.Old), editor.NewBuffer(comparison.New)
			old.SetLanguage(editor.LanguageForText(path, comparison.Old))
			next.SetLanguage(editor.LanguageForText(path, comparison.New))
			d.comparison = &editor.DiffState{Old: old, New: next, Layout: editor.DiffSplit}
			d.binary = comparison.Binary
		}, e
	})
}
func (d *Desktop) workspace(c *native.Context, t *native.Theme) {
	files := d.visible()
	native.Row(c).Grow(1).FillWidth().Gap(14).AlignItems(native.Stretch).Children(func() {
		native.Column(c).Width(360).FillHeight().Gap(10).Children(func() {
			native.Row(c).Gap(8).AlignItems(native.Center).Children(func() {
				native.Textf(c, d.text("changes"), len(files)).Grow(1)
				if iconButton(c, d.text("stage.all"), "stage", false).Disabled(d.busy || len(files) == 0).Clicked() {
					paths := []string{}
					for _, f := range files {
						paths = append(paths, f.Path)
						if f.OriginalPath != "" {
							paths = append(paths, f.OriginalPath)
						}
					}
					d.mutate("git.stage", ipc.Params{Paths: paths})
				}
				if iconButton(c, "", "refresh", false).Tooltip(d.text("refresh")).Label(d.text("refresh")).Disabled(d.busy).Clicked() {
					d.refresh()
				}
			})
			native.TextArea(c, &d.message).Placeholder(d.text("commit.message")).FillWidth().Height(72).Disabled(d.busy)
			native.Row(c).Gap(8).Children(func() {
				label := d.text("identity.edit")
				if d.identity.EffectiveName != "" {
					label = d.identity.EffectiveName + " <" + d.identity.EffectiveEmail + ">"
				}
				if iconButton(c, label, "user", false).Grow(1).Disabled(d.busy).Clicked() {
					d.page = 4
					d.refresh()
				}
				if iconButton(c, d.text("commit.submit"), "commit", true).Disabled(d.busy || d.stagedCount == 0 || strings.TrimSpace(d.message) == "").Clicked() {
					d.mutate("git.commit", ipc.Params{Message: d.message})
				}
			})
			d.files.Selected = &d.row
			previous := d.row
			native.Table(c, &d.files, []native.TableColumn{{Title: d.text("file")}, {Title: d.text("index"), Width: 88}, {Title: d.text("worktree"), Width: 88}}, len(files), func(row, col int) {
				f := files[row]
				if col == 0 {
					native.Row(c).Gap(6).AlignItems(native.Center).Children(func() {
						native.Icon(c, icons["file"]).Width(14).Height(14).TextColor(t.TextMuted)
						native.Text(c, f.Path)
					})
				} else if col == 1 {
					native.Text(c, d.text("state."+string(f.Index)))
				} else {
					native.Text(c, d.text("state."+string(f.Worktree)))
				}
			}).Grow(1).FillWidth().Disabled(d.busy)
			if d.row != previous && d.row >= 0 && d.row < len(files) {
				d.staged = files[d.row].Worktree == git.Clean
				d.diff(files[d.row].Path)
			}
			native.Row(c).Gap(8).Children(func() {
				wasStaged := d.staged
				native.Checkbox(c, &d.staged, d.text("diff.staged"))
				valid := d.row >= 0 && d.row < len(files)
				if wasStaged != d.staged && valid && !d.busy {
					d.diff(files[d.row].Path)
				}
				for _, op := range []struct{ label, method string }{{"stage", "git.stage"}, {"unstage", "git.unstage"}} {
					if iconButton(c, d.text(op.label), op.label, false).Disabled(d.busy || !valid).Clicked() {
						f := files[d.row]
						paths := []string{f.Path}
						if f.OriginalPath != "" {
							paths = append(paths, f.OriginalPath)
						}
						d.mutate(op.method, ipc.Params{Paths: paths})
					}
				}
			})
		})
		native.Column(c).Grow(1).FillHeight().Gap(10).Children(func() {
			native.Row(c).Gap(8).AlignItems(native.Center).Children(func() {
				native.Text(c, func() string {
					if d.row >= 0 && d.row < len(files) {
						return files[d.row].Path
					}
					return d.text("diff.all")
				}()).Grow(1)
			})
			if d.binary {
				native.Text(c, d.text("diff.binary")).TextColor(t.TextMuted)
			} else if d.comparison != nil {
				editor.DiffView(c, d.comparison).Grow(1).FillWidth()
			} else if d.detail == "" {
				native.Column(c).Grow(1).FillWidth().Center().Children(func() { native.Text(c, d.text("diff.empty")).TextColor(t.TextMuted) })
			} else {
				native.TextArea(c, &d.detail).Grow(1).FillWidth().ReadOnly(true)
			}
		})
	})
}
func (d *Desktop) view(c *native.Context) {
	if c.Shortcut(native.Cmd, native.KeyO) {
		d.chooseRepository()
	}
	switch d.prefs.Theme {
	case "dark":
		c.SetTheme(native.DarkTheme())
	case "light":
		c.SetTheme(native.LightTheme())
	}
	t := c.Theme()
	native.Row(c).Fill().Children(func() {
		native.Column(c).Width(192).FillHeight().Padding(12).Gap(8).Background(t.Surface).Children(func() {
			native.Row(c).Gap(10).AlignItems(native.Center).Children(func() {
				native.Image(c, brandBitmap).Width(32).Height(32)
				native.Text(c, "MyGittu").FontSize(23).Bold()
			})
			if iconButton(c, d.text("repo.open")+"…", "open", true).FillWidth().Disabled(d.busy).Clicked() {
				d.chooseRepository()
			}
			if iconButton(c, d.text("clone.run")+"…", "clone", false).FillWidth().Disabled(d.busy).Clicked() {
				d.page = 5
			}
			for _, item := range []struct {
				page  int
				label string
			}{{0, "changes.nav"}, {1, "nav.history"}, {3, "nav.branches"}, {4, "nav.settings"}} {
				label := d.text(item.label)
				key := map[int]string{0: "changes", 1: "history", 3: "branch", 4: "settings"}[item.page]
				if iconButton(c, label, key, false).FillWidth().Selected(d.page == item.page || item.page == 0 && d.page == 2).Disabled(d.busy).Clicked() {
					d.page = item.page
					d.offset = 0
					d.refresh()
				}
			}
			if native.Button(c, d.text("repo.manual")).FillWidth().Disabled(d.busy).Clicked() {
				d.showPath = !d.showPath
			}
			if d.showPath {
				if native.TextInput(c, &d.path).Placeholder(d.text("repo.path")).FillWidth().Disabled(d.busy).Submitted() {
					d.open(d.path)
				}
				if native.Button(c, d.text("repo.open")).FillWidth().Disabled(d.busy || strings.TrimSpace(d.path) == "").Clicked() {
					d.open(d.path)
				}
			}
			if len(d.repos) > 0 {
				native.Text(c, d.text("repo.recent")).TextColor(t.TextMuted)
			}
			choice := d.selected
			native.Sidebar(c, &choice, func() {
				for _, r := range d.repos {
					native.SidebarItem(c, r.ID, icons["open"], filepath.Base(r.Root)).Tooltip(r.Root)
				}
			}).Grow(1).Disabled(d.busy).OnChange(func() {
				for _, repo := range d.repos {
					if repo.ID == choice {
						d.open(repo.Root)
						break
					}
				}
			})
			if native.Button(c, d.text("repo.remove")).FillWidth().Disabled(d.busy || d.selected == "").Clicked() {
				id := d.selected
				d.task(func(ctx context.Context) (func(), error) {
					e := d.Client.Call(ctx, "repo.unregister", ipc.Params{RepoID: id}, nil)
					if e != nil {
						return nil, e
					}
					var repos []repository.Info
					e = d.Client.Call(ctx, "repo.list", ipc.Params{}, &repos)
					return func() {
						d.repos = repos
						d.selected = ""
						d.snapshot = status.Snapshot{}
						if len(repos) > 0 {
							d.selected = repos[0].ID
							d.refresh()
						}
					}, e
				})
			}
		})
		native.Column(c).Grow(1).FillHeight().Padding(20).Gap(12).Children(func() {
			native.Row(c).Gap(10).Children(func() {
				for _, r := range d.repos {
					if r.ID == d.selected {
						native.Text(c, filepath.Base(r.Root)).Bold().Tooltip(r.Root)
					}
				}
				if d.snapshot.Branch != "" && (d.page == 0 || d.page == 2) {
					native.Text(c, d.snapshot.Branch).TextColor(t.TextMuted)
				}
				if d.selected != "" && d.page != 5 {
					for _, op := range []struct{ label, action string }{{"remote.fetch", "fetch"}, {"remote.pull", "pull"}, {"remote.push", "push"}} {
						if iconButton(c, d.text(op.label), op.label, false).Disabled(d.busy).Clicked() {
							d.mutate("git.remote", ipc.Params{Action: op.action})
						}
					}
				}
				native.Box(c).Grow(1)
				if iconButton(c, d.text("refresh"), "refresh", false).Disabled(d.busy || (d.selected == "" && d.page != 4)).Clicked() {
					d.refresh()
				}
			})
			if d.busy {
				native.Text(c, d.text("busy")).TextColor(t.TextMuted)
			}
			if d.update.Available {
				if native.Button(c, fmt.Sprintf(d.text("update.available"), d.update.Latest)).Clicked() {
					c.OpenURL(d.update.URL)
				}
			}
			if d.err != "" {
				native.Text(c, d.text("error")+": "+d.err).TextColor(native.Hex("#bf3030"))
			}
			if d.selected == "" && d.page != 4 && d.page != 5 {
				native.Column(c).Grow(1).FillWidth().Center().Gap(16).Children(func() {
					native.Text(c, "MyGittu").FontSize(32).Bold()
					native.Text(c, d.text("repo.drop")).TextColor(t.TextMuted)
					if iconButton(c, d.text("repo.open")+"…", "open", true).Disabled(d.busy).Clicked() {
						d.chooseRepository()
					}
				})
				return
			}
			switch d.page {
			case 0, 2:
				d.workspace(c, t)
			case 1:
				native.Row(c).Gap(8).Children(func() {
					native.TextInput(c, &d.query).Placeholder(d.text("history.search")).Grow(1).Disabled(d.busy)
					if iconButton(c, d.text("search"), "search", false).Disabled(d.busy).Clicked() {
						d.offset = 0
						d.refresh()
					}
				})
				d.history.Selected = &d.logRow
				native.Table(c, &d.history, []native.TableColumn{{Title: d.text("nav.commit"), Width: 110}, {Title: d.text("subject")}, {Title: d.text("author"), Width: 130}, {Title: d.text("date"), Width: 190}}, len(d.logs), func(row, col int) {
					l := d.logs[row]
					values := []string{l.Hash[:min(10, len(l.Hash))], l.Subject, l.Author, l.Date}
					native.Text(c, values[col])
				}).Grow(1).FillWidth()
				if native.Button(c, d.text("commit.details")).Disabled(d.busy || d.logRow < 0 || d.logRow >= len(d.logs)).Clicked() {
					id, hash := d.selected, d.logs[d.logRow].Hash
					d.task(func(ctx context.Context) (func(), error) {
						var detail string
						e := d.Client.Call(ctx, "git.show", ipc.Params{RepoID: id, Hash: hash}, &detail)
						return func() { d.detail = detail }, e
					})
				}
				if d.detail != "" {
					native.TextArea(c, &d.detail).Height(180).FillWidth().ReadOnly(true)
				}
				native.Row(c).Gap(8).Children(func() {
					if native.Button(c, d.text("previous")).Disabled(d.busy || d.offset == 0).Clicked() {
						d.offset = max(0, d.offset-100)
						d.refresh()
					}
					if len(d.logs) == 0 {
						native.Text(c, d.text("history.empty"))
					} else {
						native.Text(c, fmt.Sprintf("%d–%d", d.offset+1, d.offset+len(d.logs)))
					}
					if native.Button(c, d.text("next")).Disabled(d.busy || len(d.logs) < 100).Clicked() {
						d.offset += 100
						d.refresh()
					}
				})
			case 3:
				native.TextArea(c, &d.detail).Grow(1).FillWidth().ReadOnly(true)
				native.TextInput(c, &d.branch).Placeholder(d.text("branch.name")).FillWidth()
				native.Row(c).Gap(8).Children(func() {
					for _, op := range []struct{ label, action string }{{d.text("branch.create"), "create"}, {d.text("branch.switch"), "switch"}, {d.text("branch.delete"), "delete"}, {d.text("branch.merge"), "merge"}} {
						if native.Button(c, op.label).Disabled(d.busy || d.branch == "").Clicked() {
							d.mutate("git.branch", ipc.Params{Action: op.action, Name: d.branch})
						}
					}
				})
			case 4:
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
			case 5:
				native.Text(c, d.text("clone.title")).Bold()
				native.TextInput(c, &d.cloneURL).Placeholder(d.text("clone.url")).FillWidth()
				native.TextInput(c, &d.cloneDestination).Placeholder(d.text("clone.destination")).FillWidth()
				if native.PrimaryButton(c, d.text("clone.run")).Disabled(d.busy || strings.TrimSpace(d.cloneURL) == "" || !filepath.IsAbs(d.cloneDestination)).Clicked() {
					url, dest := d.cloneURL, d.cloneDestination
					d.task(func(ctx context.Context) (func(), error) {
						var repo repository.Info
						e := d.Client.Call(ctx, "git.clone", ipc.Params{URL: url, Destination: dest}, &repo)
						return func() { d.repos = append(d.repos, repo); d.selected = repo.ID; d.page = 0; d.refresh() }, e
					})
				}
			case 6:
				native.Text(c, d.text("remote."+d.branch)).Bold()
				if native.PrimaryButton(c, d.text("remote."+d.branch)).Disabled(d.busy).Clicked() {
					action := d.branch
					d.page = 0
					d.mutate("git.remote", ipc.Params{Action: action})
				}
			}
		})
	})
}
