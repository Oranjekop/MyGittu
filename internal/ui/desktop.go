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
	"mygittu/internal/platform"
	"mygittu/internal/repository"
	"mygittu/internal/settings"
	"mygittu/internal/status"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type Desktop struct {
	Client                             *ipc.Client
	win                                *mygo.Window
	repos                              []repository.Info
	selected                           string
	path, message, branch, detail, err string
	draftDir, savedMessage             string
	drafts                             map[string]string
	statusPolling                      bool
	page                               int
	busy                               bool
	operation                          string
	operationFrame                     int
	snapshot                           status.Snapshot
	logs                               []app.Commit
	row                                int
	files, history                     native.ListState
	filePane                           float32
	sidebarCollapsed                   bool
	captionStyled, captionDark         bool
	staged                             bool
	offset                             int
	query                              string
	cloneURL, cloneDestination         string
	logRow                             int
	changes                            []git.FileStatus
	stagedCount                        int
	prefs                              settings.Values
	settingsTab                        int
	aiPromptDraft                      string
	aiForm                             aiSettingsForm
	aiConfirm, aiGenerating            bool
	aiError                            string
	languageIndex, themeIndex          int
	identity                           app.Identity
	commitIdentity                     app.Identity
	identityScope                      int
	identitySaved                      bool
	update                             app.UpdateInfo
	updateChecking                     bool
	updateInstalling                   bool
	updateCancel                       context.CancelFunc
	updateBytes, updateTotal           int64
	updateError                        string
	settingsOpen                       bool
	repositoriesOpen                   bool
	repositorySearch                   string
	comparison                         *editor.DiffState
	binary                             bool
	projectFiles                       []string
	projectSearch                      string
	projectTree                        []projectEntry
	projectCollapsed                   map[string]bool
	projectFileRow                     int
	projectFileList                    native.ListState
	projectPreview                     app.Comparison
	tools                              workbench
	references                         app.References
	stashRow, tagRow                   int
	stashList, tagList                 native.ListState
	stashMessage, tagName              string
	hunksOpen                          bool
	hunkPath                           string
	hunks                              []app.StageHunk
	hunkRow                            int
	hunkList                           native.ListState
}

func New(client *ipc.Client, path, action string) *Desktop {
	d := &Desktop{Client: client, path: path, row: -1, logRow: -1, filePane: 310, prefs: settings.Values{Language: "zh-CN", Theme: "system"}}
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
		if dir, err := platform.ConfigDir(); err == nil {
			d.draftDir = filepath.Join(dir, "drafts")
		}
		d.win = mygo.NewWindow(mygo.WindowOptions{Title: "MyGittu", Width: 1280, Height: 820, MinWidth: 960, MinHeight: 680, Content: native.View(d.view)})
		_ = d.win.SetIcon(brand.PNG)
		_ = mygo.App.Dock.SetIcon(brand.PNG)
		updateContext, cancelUpdates := context.WithCancel(context.Background())
		d.win.OnClosed(func() {
			cancelUpdates()
			if d.updateCancel != nil {
				d.updateCancel()
			}
			d.saveDraft()
		})
		go d.watchUpdates(updateContext)
		go app.CleanupUpdates()
		go d.watchWorkspace(updateContext)
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
				return func() { d.applyPrefs(prefs); d.repos = repos; d.selectRepository(repo.ID, repo.Root); d.refresh() }, e
			}
			return func() {
				d.applyPrefs(prefs)
				d.repos = repos
				if len(repos) > 0 && repos[0].LastOpened > 0 {
					d.selectRepository(repos[0].ID, repos[0].Root)
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
	previousAI := d.prefs.AI
	previousLanguage := d.prefs.Language
	d.prefs = v
	if previousLanguage != v.Language && v.AIPrompt == "" {
		d.resetPromptDraft()
	}
	if previousAI != v.AI {
		d.aiForm.apply(v.AI)
	}
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
	d.operationTask("", work)
}

func (d *Desktop) operationTask(operation string, work func(context.Context) (func(), error)) {
	if d.busy {
		return
	}
	d.busy = true
	d.err = ""
	d.operation, d.operationFrame = operation, 0
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if operation != "" {
			go d.animateOperation(ctx)
		}
		apply, e := work(ctx)
		mygo.RunOnMain(func() {
			d.busy = false
			d.operation, d.operationFrame = "", 0
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
	if id == "" {
		return
	}
	d.row = -1
	d.detail = ""
	d.comparison = nil
	if page == projectFilesPage {
		d.projectFiles, d.projectFileRow, d.projectPreview = nil, -1, app.Comparison{}
		d.projectTree = nil
	}
	d.task(func(ctx context.Context) (func(), error) {
		var snap status.Snapshot
		var logs []app.Commit
		var projectFiles []string
		var references app.References
		var branchTargets []string
		var branches []app.BranchEntry
		var detail string
		var e error
		var identity app.Identity
		switch page {
		case 0, 2:
			e = d.Client.Call(ctx, "status.refresh", ipc.Params{RepoID: id}, &snap)
			if e == nil {
				e = d.Client.Call(ctx, "identity.get", ipc.Params{RepoID: id, Scope: "local"}, &identity)
			}
		case 1:
			e = d.Client.Call(ctx, "git.log", ipc.Params{RepoID: id, Limit: 100, Offset: offset, Query: query}, &logs)
		case 3:
			e = d.Client.Call(ctx, "git.branch.entries", ipc.Params{RepoID: id}, &branches)
			for _, branch := range branches {
				branchTargets = append(branchTargets, branch.Name)
			}
		case projectFilesPage:
			e = d.Client.Call(ctx, "git.files", ipc.Params{RepoID: id}, &projectFiles)
		case referencesPage:
			e = d.Client.Call(ctx, "git.references", ipc.Params{RepoID: id}, &references)
		}
		return func() {
			if page == 3 {
				d.tools.branches = branches
				if !slices.Contains(branchTargets, d.tools.branchSelected) {
					d.tools.branchSelected = ""
					for _, branch := range branches {
						if branch.Current {
							d.tools.branchSelected = branch.Name
							break
						}
					}
				}
				d.tools.branchTargets = branchTargets
				if !slices.Contains(branchTargets, d.tools.left) && len(branchTargets) > 0 {
					d.tools.left = branchTargets[0]
				}
				if !slices.Contains(branchTargets, d.tools.right) && len(branchTargets) > 1 {
					d.tools.right = branchTargets[1]
				}
			}
			if page == referencesPage {
				d.references, d.stashRow, d.tagRow = references, -1, -1
				return
			}
			if page == projectFilesPage {
				d.projectFiles, d.projectFileRow, d.projectPreview = projectFiles, -1, app.Comparison{}
				d.rebuildProjectTree()
				return
			}
			if page == 0 || page == 2 {
				d.identity = editableIdentity(identity)
				d.commitIdentity = identity
				oldMode := d.staged
				changed := d.applyWorkspaceStatus(snap)
				if d.row >= 0 && (changed || d.staged != oldMode) {
					d.diff(d.changes[d.row].Path)
				}
			}
			d.logs = logs
			d.detail = detail
			d.logRow = -1
		}, e
	})
}
func (d *Desktop) mutate(method string, p ipc.Params) {
	p.RepoID = d.selected
	operation := ""
	if method == "git.commit" {
		operation = "commit.running"
	}
	d.operationTask(operation, func(ctx context.Context) (func(), error) {
		e := d.Client.Call(ctx, method, p, nil)
		return func() {
			if method == "git.commit" {
				d.message = ""
				d.saveDraft()
			}
			if method == "git.stage" || method == "git.unstage" {
				clear(d.tools.batch)
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
			d.comparison = nil
			d.applyComparison(path, comparison)
		}, e
	})
}
func (d *Desktop) applyComparison(path string, comparison app.Comparison) bool {
	changed := d.binary != comparison.Binary
	d.binary = comparison.Binary
	if d.comparison == nil {
		old, next := editor.NewBuffer(comparison.Old), editor.NewBuffer(comparison.New)
		old.SetLanguage(editor.LanguageForText(path, comparison.Old))
		next.SetLanguage(editor.LanguageForText(path, comparison.New))
		d.comparison = &editor.DiffState{Old: old, New: next, Layout: editor.DiffSplit}
		return true
	}
	if d.comparison.Old.String() != comparison.Old {
		d.comparison.Old.Reset(comparison.Old)
		changed = true
	}
	if d.comparison.New.String() != comparison.New {
		d.comparison.New.Reset(comparison.New)
		changed = true
	}
	return changed
}

func (d *Desktop) workspace(c *native.Context, t *native.Theme) {
	files := d.visible()
	width, _ := c.Size()
	available := width - 2*panelSpacing
	if !d.sidebarCollapsed {
		available -= sidebarWidth + panelSpacing
	}
	fileWidth := min(max(d.filePane, 284), max(284, available-320))
	dividerTheme := *t
	dividerTheme.Border = t.Surface
	dividerTheme.Accent = t.Border
	pane := func(right bool, contents func()) {
		container := native.Themed(c, t).Fill().Background(t.Surface)
		if right {
			container.PaddingStart(panelSpacing / 2)
		} else {
			container.PaddingEnd((panelSpacing - 1) / 2)
		}
		container.Children(contents)
	}
	canCommit := d.stagedCount > 0 && !slices.ContainsFunc(files, func(file git.FileStatus) bool { return file.State == git.Conflicted })
	filesPane := func() {
		native.Column(c).Fill().Padding(panelSpacing).Gap(controlSpacing).Background(t.Background).Radius(8).Children(func() {
			native.Row(c).Gap(controlSpacing).AlignItems(native.Center).Children(func() {
				native.Textf(c, d.text("changes"), len(files)).Grow(1)
				canStage := slices.ContainsFunc(files, func(file git.FileStatus) bool {
					return file.State != git.Conflicted && file.Worktree != git.Clean && file.Worktree != ""
				})
				if len(d.tools.batch) == 0 && IconButton(c, d.text("stage.all"), "stage", false).Disabled(d.busy || !canStage).Clicked() {
					var paths []string
					for _, file := range files {
						paths = stagingPaths(paths, file, true)
					}
					d.mutate("git.stage", ipc.Params{Paths: paths})
				}
			})
			if d.snapshot.Merging && !slices.ContainsFunc(files, func(file git.FileStatus) bool { return file.State == git.Conflicted }) {
				if IconButton(c, d.text("recover.merge.continue"), "merge", false).Disabled(d.busy).Clicked() {
					d.confirmRecovery("merge.continue", "")
				}
			}
			d.files.Selected = &d.row
			previous := d.row
			d.showStagingActions(c, files)
			native.Table(c, &d.files, []native.TableColumn{{Title: d.text("file")}, {Title: d.text("index"), Width: 68}, {Title: d.text("worktree"), Width: 68}}, len(files), func(row, col int) {
				f := files[row]
				if col == 0 {
					native.Row(c).Gap(controlSpacing).AlignItems(native.Center).Children(func() {
						checked := d.tools.batch[f.Path]
						native.Checkbox(c, &checked, "").Label(d.copy("批量选择 ", "Select for batch ") + f.Path)
						if checked != d.tools.batch[f.Path] {
							if checked {
								if d.tools.batch == nil {
									d.tools.batch = map[string]bool{}
								}
								d.tools.batch[f.Path] = true
							} else {
								delete(d.tools.batch, f.Path)
							}
						}
						native.Icon(c, icons["file"]).Width(14).Height(14).TextColor(t.TextMuted)
						native.Row(c).Grow(1).MinWidth(0).Gap(detailSpacing).AlignItems(native.Center).Tooltip(f.Path).Children(func() {
							dir, name := filepath.Split(f.Path)
							native.Text(c, name).MinWidth(0).SingleLine().Ellipsis("…")
							if dir != "" {
								native.Text(c, strings.TrimRight(dir, "/")).Grow(1).MinWidth(0).SingleLine().Ellipsis("…").TextColor(t.TextMuted)
							}
						})
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
			if d.aiError != "" {
				native.Text(c, d.aiError).FillWidth().TextColor(t.Danger)
			}
			native.TextArea(c, &d.message).Placeholder(d.text("commit.message")).FillWidth().MinHeight(46).Height(46).Disabled(d.busy)
			native.Row(c).Gap(controlSpacing).Wrap().Children(func() {
				if IconButton(c, "AI", "sparkle", false).Label(d.copy("AI 生成提交信息", "Generate commit message with AI")).Tooltip(d.copy("AI 生成提交信息", "Generate commit message with AI")).Disabled(d.busy || !canCommit).Clicked() {
					d.requestCommitAI()
				}
				label := d.text("identity.edit")
				if d.commitIdentity.EffectiveName != "" {
					label = d.commitIdentity.EffectiveName
				}
				if IconButton(c, label, "user", false).Grow(1).MinWidth(0).Tooltip(d.commitIdentity.EffectiveEmail).Disabled(d.busy).Clicked() {
					d.identityScope = 0
					d.openSettings()
					d.settingsTab = 1
					d.loadIdentity()
				}
				if IconButton(c, d.text("commit.submit"), "commit", true).Disabled(d.busy || !canCommit || strings.TrimSpace(d.message) == "").Clicked() {
					d.mutate("git.commit", ipc.Params{Message: d.message})
				}
			})
		})
	}
	diffPane := func() {
		native.Column(c).Fill().Padding(panelSpacing).Gap(controlSpacing).Background(t.Background).Radius(8).Children(func() {
			native.Row(c).Gap(controlSpacing).AlignItems(native.Center).Children(func() {
				native.Text(c, func() string {
					if d.row >= 0 && d.row < len(files) {
						return files[d.row].Path
					}
					return d.text("diff.all")
				}()).Grow(1)
				if d.row >= 0 && d.row < len(files) {
					file := files[d.row]
					if file.Index != git.Clean && file.Worktree != git.Clean && file.State != git.Conflicted {
						mode := 0
						if d.staged {
							mode = 1
						}
						previous := mode
						native.Segmented(c, &mode, d.text("worktree"), d.text("index")).Disabled(d.busy)
						if mode != previous {
							d.staged = mode == 1
							d.diff(file.Path)
						}
					}
				}
				if d.row >= 0 && d.row < len(files) && files[d.row].State == git.Conflicted {
					if IconButton(c, d.text("conflict.open"), "conflict", false).Disabled(d.busy).Clicked() {
						d.loadConflict(files[d.row].Path)
					}
				}
				if d.row >= 0 && d.row < len(files) && !d.staged && files[d.row].Worktree == git.Modified {
					if IconButton(c, d.text("hunk.stage"), "stage", false).Disabled(d.busy).Clicked() {
						d.loadHunks(files[d.row].Path)
					}
				}
			})
			if d.binary {
				native.Text(c, d.text("diff.binary")).TextColor(t.TextMuted)
			} else if d.comparison != nil {
				d.comparison.Font.Size = t.FontSize - 0.5
				d.comparison.Font.Family = c.Theme().Font
				d.comparison.Layout = editor.DiffSplit
				if available-fileWidth < 550 {
					d.comparison.Layout = editor.DiffUnified
				}
				editor.DiffView(c, d.comparison).Grow(1).FillWidth()
			} else if d.detail == "" {
				native.Column(c).Grow(1).FillWidth().Center().Children(func() { native.Text(c, d.text("diff.empty")).TextColor(t.TextMuted) })
			} else {
				native.TextArea(c, &d.detail).Grow(1).FillWidth().ReadOnly(true)
			}
		})
	}
	native.Themed(c, &dividerTheme).Grow(1).FillWidth().Background(t.Surface).Children(func() {
		split := native.Split(c, &fileWidth, func() { pane(false, filesPane) }, func() { pane(true, diffPane) }).
			Grow(1).FillWidth().SplitReset(310)
		if split.Changed() {
			d.filePane = fileWidth
		}
	})
}
func (d *Desktop) view(c *native.Context) {
	if !d.settingsOpen && !d.repositoriesOpen && c.Shortcut(native.Cmd, native.KeyO) {
		d.chooseRepository()
	}
	if !d.settingsOpen && !d.busy && c.Shortcut(native.Cmd, native.KeyK) {
		d.repositorySearch = ""
		d.repositoriesOpen = true
	}
	switch d.prefs.Theme {
	case "dark":
		c.SetTheme(native.DarkTheme())
	case "light":
		c.SetTheme(native.LightTheme())
	}
	t := ApplyTheme(c, d.prefs.TextSize())
	if d.win != nil && (!d.captionStyled || d.captionDark != t.Dark) {
		if handle := d.win.NativeHandle(); handle != 0 {
			StyleCaption(handle, t.Surface, t.Text)
			d.captionStyled, d.captionDark = true, t.Dark
		}
	}
	native.Column(c).Fill().Padding(panelSpacing).Gap(panelSpacing).Background(t.Surface).Children(func() {
		native.Row(c).FillWidth().Gap(controlSpacing).AlignItems(native.Center).Children(func() {
			if IconButton(c, "", "diff", false).Label(d.text("nav.toggle")).Tooltip(d.text("nav.toggle")).Clicked() {
				d.sidebarCollapsed = !d.sidebarCollapsed
			}
			d.repositoryMenu(c)
			if d.snapshot.Branch != "" && (d.page == 0 || d.page == 2) {
				native.Text(c, d.snapshot.Branch).TextColor(t.TextMuted)
				tracking := d.snapshot.Tracking
				if tracking.Upstream == "" {
					native.Text(c, d.text("branch.noUpstream")).TextColor(t.TextMuted)
				} else if !tracking.Compared {
					native.Text(c, d.text("branch.upstreamUnavailable")).Tooltip(tracking.Upstream).TextColor(t.TextMuted)
				} else {
					native.Textf(c, "↑ %d  ↓ %d", tracking.Ahead, tracking.Behind).Tooltip(tracking.Upstream).TextColor(t.TextMuted)
				}
			}
			native.Box(c).Grow(1)
			if d.selected != "" && d.page != 5 {
				for _, op := range []struct{ label, action string }{{"remote.fetch", "fetch"}, {"remote.pull", "pull"}, {"remote.push", "push"}} {
					if IconButton(c, d.text(op.label), op.label, false).Disabled(d.busy).Clicked() {
						d.remoteAction(op.action)
					}
				}
			}
			if IconButton(c, "", "refresh", false).Label(d.text("refresh")).Tooltip(d.text("refresh")).Disabled(d.busy || (d.selected == "" && d.page != 4)).Clicked() {
				d.refresh()
			}
			if IconButton(c, "", "settings", false).Label(d.text("nav.settings")).Tooltip(d.text("nav.settings")).Clicked() {
				d.openSettings()
			}
		})
		native.Row(c).Grow(1).FillWidth().Gap(panelSpacing).Children(func() {
			if !d.sidebarCollapsed {
				native.Column(c).Width(sidebarWidth).FillHeight().Padding(controlSpacing).Gap(detailSpacing).Background(t.Background).Radius(8).Children(func() {
					for _, item := range []struct {
						page        int
						label, icon string
					}{
						{0, "changes.nav", "changes"},
						{projectFilesPage, "nav.files", "file"},
						{1, "nav.history", "history"},
						{3, "nav.branches", "branch"},
						{referencesPage, "nav.references", "stash"},
					} {
						label := d.text(item.label)
						active := d.page == item.page || item.page == 0 && d.page == 2
						if IconButton(c, "", item.icon, active).Label(label).Tooltip(label).FillWidth().Disabled(d.busy).Clicked() {
							d.page, d.offset = item.page, 0
							d.refresh()
						}
					}
					native.Box(c).Grow(1)
					if IconButton(c, "", "open", false).Label(d.text("repo.open")).Tooltip(d.text("repo.open")).FillWidth().Disabled(d.busy).Clicked() {
						d.chooseRepository()
					}
					if IconButton(c, "", "clone", d.page == 5).Label(d.text("clone.run")).Tooltip(d.text("clone.run")).FillWidth().Disabled(d.busy).Clicked() {
						d.page = 5
					}
				})
			}
			native.Column(c).Grow(1).FillHeight().Gap(controlSpacing).Children(func() {

				if d.busy && !d.settingsOpen {
					native.Row(c).Gap(controlSpacing).AlignItems(native.Center).Children(func() {
						if d.operation == "" {
							label := d.text("busy")
							if d.aiGenerating {
								label = d.aiActionLabel()
							}
							native.Text(c, label).TextColor(t.TextMuted)
						} else {
							action, _, _ := strings.Cut(d.operation, ".")
							OperationIcon(c, 40, d.operationFrame, action)
							native.Text(c, d.text(d.operation)).TextColor(t.TextMuted)
						}
					})
				}
				if d.update.Available {
					label := fmt.Sprintf(d.text("update.install"), d.update.Latest)
					if d.updateInstalling {
						label = d.text("update.downloading")
					}
					if IconButton(c, label, "download", false).Disabled(d.updateInstalling).Clicked() {
						d.installUpdate()
					}
				}
				if d.err != "" {
					native.Text(c, d.text("error")+": "+d.err).TextColor(native.Hex("#bf3030"))
				}
				if d.selected == "" && d.page != 4 && d.page != 5 {
					native.Column(c).Grow(1).FillWidth().Center().Gap(16).Children(func() {
						native.Text(c, "MyGittu").FontSize(28).Bold()
						native.Text(c, d.text("repo.drop")).TextColor(t.TextMuted)
						if IconButton(c, d.text("repo.open")+"…", "open", true).Disabled(d.busy).Clicked() {
							d.chooseRepository()
						}
					})
					return
				}
				switch d.page {
				case referencesPage:
					d.showReferences(c, t)
				case projectFilesPage:
					d.showProjectFiles(c, t)
				case 0, 2:
					d.workspace(c, t)
				case 1:
					native.Row(c).Gap(controlSpacing).Children(func() {
						native.TextInput(c, &d.query).Placeholder(d.text("history.search")).Grow(1).Disabled(d.busy)
						if IconButton(c, d.text("search"), "search", false).Disabled(d.busy).Clicked() {
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
					if IconButton(c, d.text("commit.details"), "changes", false).Disabled(d.busy || d.logRow < 0 || d.logRow >= len(d.logs)).Clicked() {
						id, hash := d.selected, d.logs[d.logRow].Hash
						d.task(func(ctx context.Context) (func(), error) {
							var detail string
							e := d.Client.Call(ctx, "git.show", ipc.Params{RepoID: id, Hash: hash}, &detail)
							return func() { d.detail = detail }, e
						})
					}
					native.Row(c).Gap(controlSpacing).Children(func() {
						valid := d.logRow >= 0 && d.logRow < len(d.logs)
						if IconButton(c, d.text("recover.undo"), "undo", false).Disabled(d.busy || !valid || d.offset != 0 || d.query != "" || d.logRow != 0).Clicked() && valid {
							d.confirmRecovery("undo", d.logs[d.logRow].Hash)
						}
						if IconButton(c, d.text("recover.revert"), "revert", false).Disabled(d.busy || !valid).Clicked() && valid {
							d.confirmRecovery("revert", d.logs[d.logRow].Hash)
						}
					})
					if d.detail != "" {
						native.TextArea(c, &d.detail).Height(180).FillWidth().ReadOnly(true)
					}
					native.Row(c).Gap(controlSpacing).Children(func() {
						if IconButton(c, d.text("previous"), "left", false).Disabled(d.busy || d.offset == 0).Clicked() {
							d.offset = max(0, d.offset-100)
							d.refresh()
						}
						if len(d.logs) == 0 {
							native.Text(c, d.text("history.empty"))
						} else {
							native.Text(c, fmt.Sprintf("%d–%d", d.offset+1, d.offset+len(d.logs)))
						}
						if IconButton(c, d.text("next"), "right", false).Disabled(d.busy || len(d.logs) < 100).Clicked() {
							d.offset += 100
							d.refresh()
						}
					})
				case 3:
					d.showBranches(c)
				case 5:
					native.Text(c, d.text("clone.title")).Bold()
					native.TextInput(c, &d.cloneURL).Placeholder(d.text("clone.url")).FillWidth()
					native.TextInput(c, &d.cloneDestination).Placeholder(d.text("clone.destination")).FillWidth()
					if IconButton(c, d.text("clone.run"), "clone", true).Disabled(d.busy || strings.TrimSpace(d.cloneURL) == "" || !filepath.IsAbs(d.cloneDestination)).Clicked() {
						url, dest := d.cloneURL, d.cloneDestination
						d.operationTask("clone.running", func(ctx context.Context) (func(), error) {
							var repo repository.Info
							e := d.Client.Call(ctx, "git.clone", ipc.Params{URL: url, Destination: dest}, &repo)
							return func() {
								d.repos = append(d.repos, repo)
								d.selectRepository(repo.ID, repo.Root)
								d.page = 0
								d.refresh()
							}, e
						})
					}
				case 6:
					native.Text(c, d.text("remote."+d.branch)).Bold()
					if IconButton(c, d.text("remote."+d.branch), d.branch, true).Disabled(d.busy).Clicked() {
						action := d.branch
						d.page = 0
						d.mutate("git.remote", ipc.Params{Action: action})
					}
				}
			})
		})
	})
	d.settingsDialog(c)
	d.commitAIDialog(c)
	d.repositoryDialog(c)
	d.hunkDialog(c)
	d.showConflict(c)
	d.showFileHistory(c)
	d.showRecovery(c)
}
