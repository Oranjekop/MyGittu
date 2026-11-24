package ui

import (
	"context"
	"fmt"
	"github.com/egoist/mygo"
	native "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"mygittu/internal/app"
	git "mygittu/internal/git"
	"mygittu/internal/i18n"
	"mygittu/internal/ipc"
	"mygittu/internal/platform"
	"mygittu/internal/repository"
	"mygittu/internal/settings"
	"mygittu/internal/status"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

type Desktop struct {
	Client                             *ipc.Client
	win                                *mygo.Window
	tray                               *mygo.Tray
	action, initialDiffPath, result    string
	offerCommitPush                    bool
	repos                              []repository.Info
	selected                           string
	path, message, branch, detail, err string
	pendingLaunchPath                  string
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
	stagedCollapsed, changesCollapsed  bool
	remoteRunning, remoteNotice        string
	remoteDiverged                     bool
	pushRetryRepo, pushFailure         string
	filePane                           float32
	filePaneChanged                    bool
	projectPane                        float32
	projectPaneChanged                 bool
	captionStyled, captionDark         bool
	staged                             bool
	query                              string
	cloneURL, cloneDestination         string
	cloneOpen                          bool
	clonedRepo                         *repository.Info
	cloneProvider                      int
	cloneParent, cloneAccount          string
	cloneRepoSearch, cloneRepoChoice   string
	cloneRepos                         []string
	cloneCredentials                   cloneCredentials
	logRow                             int
	historySummary                     *string
	historyCurrentOnly                 bool
	historyGraph                       []commitGraphRow
	historyGraphWidth                  float32
	historyGraphBuilder                commitGraphBuilder
	historyQuery                       string
	historyAllBranches                 bool
	historyMore, historyLoading        bool
	historyLoadError                   string
	historyGeneration                  uint64
	historyShown                       app.Commit
	historyPatch                       patchView
	historyMode                        int
	historySplit                       float32
	historyFiles                       []editor.FilePatch
	historyFileRow                     int
	historyFileList                    native.ListState
	historyFileDiff                    editor.DiffState
	changes                            []git.FileStatus
	stagedCount                        int
	prefs                              settings.Values
	settingsTab                        int
	repositoryTools                    repositoryTools
	integration                        integrationState
	integrationLoaded                  bool
	integrationError                   string
	aiPromptDraft                      string
	aiForm                             aiSettingsForm
	aiGenerating                       bool
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
	comparisonPath                     string
	comparisonLimited                  bool
	comparisonError                    string
	comparisonStaged                   bool
	diffLayout                         int
	diffWrap, diffExpand               bool
	workspacePatch                     *editor.State
	changeTabs                         []changeTab
	changeTabBar, projectTabBar        fileTabBarState
	binary                             bool
	projectFiles                       []string
	projectSearch                      string
	projectTree                        []projectEntry
	projectExpanded                    map[string]bool
	projectFileRow                     int
	projectFileList                    native.ListState
	projectPreview                     app.FilePreview
	projectTabs                        map[string][]*projectTab
	projectAllowClose                  bool
	projectShownPath, projectLoadError string
	projectFailedPath                  string
	projectLoad                        *projectPreviewRequest
	projectLargeText                   bool
	projectBitmap                      *native.Bitmap
	projectCode                        *editor.State
	projectCodePath, projectCodeSource string
	projectWrap, projectFormatted      bool
	projectFormatError                 string
	tools                              workbench
	references                         app.References
	stashRow, tagRow                   int
	stashPreview                       stashPreview
	fileSearch                         projectContentSearch
	stashList, tagList                 native.ListState
	tagSort, tagAppliedSort            native.SortOrder
	tagOrder                           []string
	stashMessage, tagName              string
	hunksOpen                          bool
	hunkPath                           string
	hunks                              []app.StageHunk
	hunkRow                            int
	hunkList                           native.ListState
	hunkPatch                          patchView
	hunkPreviewRow                     int
	hunkConfirm                        bool
}

func New(client *ipc.Client, path, action string) *Desktop {
	d := &Desktop{Client: client, path: path, row: -1, logRow: -1, filePane: defaultFilePaneWidth,
		projectPane: defaultProjectPaneWidth, prefs: settings.Values{Language: "zh-CN", Theme: "system"}}
	d.stagedCollapsed, d.changesCollapsed = true, true
	switch action {
	case "log":
		d.action = action
		d.page = 1
	case "diff":
		d.action = action
		d.page = 2
	case "clone":
		d.action = action
		if path != "" {
			d.cloneParent = path
		}
	case "pull", "push":
		d.page = 6
		d.branch = action
	case "commit":
		d.action = action
	case "status", "":
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
		options := d.windowOptions()
		layoutPath := filePanePath(options.StateKey)
		projectLayoutPath := filePanePath(options.StateKey + "-project")
		restoreFilePane(layoutPath, &d.filePane)
		restoreFilePane(projectLayoutPath, &d.projectPane)
		d.win = NewRememberedWindow(options)
		isMac := runtime.GOOS == "darwin"
		canHideOnClose := d.action == "" && supportsHideOnClose()
		quitting := false
		if canHideOnClose {
			offQuit := mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { quitting = true })
			offCanceled := mygo.App.OnQuitCanceled(func() { quitting = false })
			if isMac {
				d.win.OnShow(d.removeTray)
				offActivate := mygo.App.OnActivate(func(hasVisibleWindows bool) {
					if !hasVisibleWindows && d.tray != nil {
						d.Activate("")
					}
				})
				d.win.OnClosed(offActivate)
			}
			d.win.OnClosed(func() {
				d.removeTray()
				offQuit()
				offCanceled()
			})
		}
		d.win.OnClose(func(e *mygo.CloseEvent) {
			if canHideOnClose && d.prefs.MinimizeOnClose && !quitting && !d.projectAllowClose {
				e.PreventDefault()
				d.saveDraft()
				if !d.ensureTray() {
					d.win.Invalidate()
					return
				}
				d.win.Hide()
				if isMac {
					mygo.App.Dock.Hide()
				}
				return
			}
			if !d.projectAllowClose && d.hasUnsavedProjectFiles() {
				e.PreventDefault()
				d.askAction(d.copy("关闭窗口并放弃未保存的文件修改？", "Close the window and discard unsaved file edits?"), "", func() {
					d.projectAllowClose = true
					d.win.Close()
				})
				d.win.Invalidate()
			}
		})
		updateContext, cancelUpdates := context.WithCancel(context.Background())
		d.win.OnClosed(func() {
			saveFilePane(layoutPath, d.filePane, d.filePaneChanged)
			saveFilePane(projectLayoutPath, d.projectPane, d.projectPaneChanged)
			d.cancelProjectPreview()
			d.fileSearch.cancelRequest()
			cancelUpdates()
			if d.updateCancel != nil {
				d.updateCancel()
			}
			d.saveDraft()
		})
		if d.action == "" {
			go d.watchUpdates(updateContext)
			go app.CleanupUpdates()
			d.win.OnFileDrop(func(e *mygo.FileDropEvent) {
				if len(e.Paths) > 0 && !d.cloneOpen {
					d.open(e.Paths[0])
				}
			})
		}
		if d.action != "clone" {
			go d.watchWorkspace(updateContext)
		}
		if d.action != "" {
			d.loadAction()
			return
		}
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
			if d.path != "" {
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
	if runtime.GOOS == "windows" && d.action == "" && d.win != nil && !d.win.IsDestroyed() {
		if v.MinimizeOnClose {
			d.ensureTray()
		} else {
			d.removeTray()
		}
	}
	if d.action != "" && previousLanguage != v.Language && d.win != nil && !d.win.IsDestroyed() {
		d.win.SetTitle(d.actionTitle())
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
		if operation != "" && operation != "commit.running" {
			go d.animateOperation(ctx, operation)
		}
		apply, e := work(ctx)
		mygo.RunOnMain(func() {
			if d.win != nil && d.win.IsDestroyed() {
				return
			}
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
			d.openPendingLaunch()
		})
	}()
}
func (d *Desktop) refresh() {
	id, page, query := d.selected, d.page, d.query
	allBranches := !d.historyCurrentOnly
	if id == "" {
		return
	}
	if page == 1 && !d.busy {
		d.invalidateHistoryLoad()
	}
	d.task(func(ctx context.Context) (func(), error) {
		var snap status.Snapshot
		var logs []app.Commit
		var projectFiles []string
		var references app.References
		var branchTargets []string
		var branches []app.BranchEntry
		var e error
		var identity app.Identity
		switch page {
		case 0, 2:
			e = d.Client.Call(ctx, "status.refresh", ipc.Params{RepoID: id}, &snap)
			if e == nil && page == 0 {
				e = d.Client.Call(ctx, "identity.get", ipc.Params{RepoID: id, Scope: "local"}, &identity)
			}
		case 1:
			e = d.Client.Call(ctx, "git.log", ipc.Params{RepoID: id, Limit: historyBatchSize + 1, Query: query, AllBranches: allBranches}, &logs)
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
				d.tagOrder = nil
				return
			}
			if page == projectFilesPage {
				d.projectFiles = projectFiles
				path := d.selectedProjectPath()
				d.rebuildProjectTree()
				if d.projectFileRow < 0 && path != "" {
					d.cancelProjectPreview()
				}
				return
			}
			if page == 0 || page == 2 {
				d.identity = editableIdentity(identity)
				d.commitIdentity = identity
				d.applyWorkspaceStatus(snap)
				if d.initialDiffPath != "" {
					target := d.initialDiffPath
					d.initialDiffPath = ""
					d.row = slices.IndexFunc(d.changes, func(file git.FileStatus) bool {
						return repository.Key(file.Path) == target
					})
					if d.row >= 0 {
						d.staged = d.changes[d.row].Worktree == git.Clean
						if d.staged {
							d.stagedCollapsed = false
						} else {
							d.changesCollapsed = false
						}
					}
				}
				if d.row >= 0 {
					d.diff(d.changes[d.row].Path)
				} else {
					d.detail, d.comparison, d.binary = "", nil, false
				}
			}
			if page == 1 {
				d.historyQuery, d.historyAllBranches = query, allBranches
				d.historyMore = len(logs) > historyBatchSize
				d.logs = logs[:min(len(logs), historyBatchSize)]
				d.historyGraphBuilder = commitGraphBuilder{}
				d.historyGraph, d.historyGraphWidth = d.historyGraphBuilder.appendRows(d.logs, query != "")
				d.history.ScrollTo(0, native.Start)
				d.logRow = slices.IndexFunc(d.logs, func(commit app.Commit) bool { return commit.Hash == d.historyShown.Hash })
				if d.logRow < 0 {
					d.historySummary = nil
					d.historyShown = app.Commit{}
				}
			}
		}, e
	})
}
func (d *Desktop) mutate(method string, p ipc.Params) {
	p.RepoID = d.selected
	d.task(func(ctx context.Context) (func(), error) {
		err := d.Client.Call(ctx, method, p, nil)
		return func() {
			if method == "git.stage" || method == "git.unstage" {
				clear(d.tools.batch)
			}
			d.refresh()
		}, err
	})
}

func (d *Desktop) commitChanges(push bool) {
	params := ipc.Params{RepoID: d.selected, Message: d.message}
	d.operationTask("commit.running", func(ctx context.Context) (func(), error) {
		if err := d.Client.Call(ctx, "git.commit", params, nil); err != nil {
			return nil, err
		}
		var pushErr error
		if push {
			pushErr = d.Client.Call(ctx, "git.remote", ipc.Params{RepoID: params.RepoID, Action: "push"}, nil)
		}
		return func() {
			d.message = ""
			d.saveDraft()
			if push {
				d.recordCommitPush(params.RepoID, pushErr)
			}
			if d.action == "commit" && pushErr == nil {
				d.result = d.copy("提交成功", "Commit completed")
				d.offerCommitPush = !push
				if push {
					d.result = d.text("commit.pushDone")
				}
				return
			}
			d.refresh()
		}, nil
	})
}
func (d *Desktop) visible() []git.FileStatus {
	return d.changes
}
func (d *Desktop) diff(path string) {
	d.rememberChangeTab(path)
	if d.comparisonPath != path || d.comparisonStaged != d.staged || d.comparisonError != "" {
		d.detail, d.comparison, d.binary = "", nil, false
		d.comparisonLimited, d.comparisonError = false, ""
	}
	if d.Client == nil {
		return
	}
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
			if e != nil {
				d.detail, d.comparison, d.binary, d.comparisonLimited, d.comparisonError = "", nil, false, false, e.Error()
				return
			}
			d.applyComparison(path, comparison)
		}, nil
	})
}
func (d *Desktop) applyComparison(path string, comparison app.Comparison) bool {
	changed := d.binary != comparison.Binary || d.comparisonLimited != comparison.Limited || d.comparisonError != ""
	d.binary = comparison.Binary
	d.comparisonLimited, d.comparisonError = comparison.Limited, ""
	if comparison.Limited {
		d.comparisonPath, d.comparisonStaged = path, d.staged
		d.comparison = nil
		return changed
	}
	if d.comparison == nil || d.comparisonPath != path || d.comparisonStaged != d.staged {
		d.comparisonPath, d.comparisonStaged = path, d.staged
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
	if d.action == "" {
		available -= sidebarWidth + panelSpacing
	}
	fileWidth := filePaneWidth(available, d.filePane)
	canCommit := d.stagedCount > 0 && !slices.ContainsFunc(files, func(file git.FileStatus) bool { return file.State == git.Conflicted })
	filesPane := func() {
		native.Column(c).Fill().Padding(panelSpacing).Gap(controlSpacing).Background(t.Background).Radius(8).Children(func() {
			if len(files) > 0 {
				d.showStagingActions(c, files)
			}
			if d.snapshot.CherryPicking {
				d.cherryControls(c, files)
			}
			if d.snapshot.Merging && !slices.ContainsFunc(files, func(file git.FileStatus) bool { return file.State == git.Conflicted }) {
				if IconButton(c, d.text("recover.merge.continue"), "merge", false).Disabled(d.busy).Clicked() {
					d.confirmRecovery("merge.continue", "")
				}
			}
			d.workspaceChangeList(c, files)
			if d.action == "diff" {
				return
			}
			if d.aiError != "" {
				native.Text(c, d.aiError).FillWidth().TextColor(t.Danger)
			}
			native.TextArea(c, &d.message).Placeholder(d.text("commit.message")).FillWidth().MinHeight(46).Height(46).Disabled(d.busy)
			native.Row(c).Gap(controlSpacing).Wrap().Children(func() {
				if activityButton(c, "AI", "sparkle", false, d.aiGenerating).Label(d.copy("AI 生成提交信息", "Generate commit message with AI")).Tooltip(d.copy("AI 生成提交信息", "Generate commit message with AI")).Disabled(d.busy || !canCommit).Clicked() {
					d.requestCommitAI()
				}
				label := d.text("identity.edit")
				if d.commitIdentity.EffectiveName != "" {
					label = d.commitIdentity.EffectiveName
				}
				if IconButton(c, label, "user", false).Grow(1).MinWidth(0).Tooltip(d.commitIdentity.EffectiveEmail).Disabled(d.busy).Clicked() {
					d.identityScope = 0
					d.openSettings()
					d.settingsTab = settingsIdentity
					d.loadIdentity()
				}
				if IconButton(c, "", "undo", false).Label(d.copy("修正最近提交", "Amend latest commit")).Tooltip(d.copy("修正最近提交", "Amend latest commit")).Disabled(d.busy || d.snapshot.Merging || d.snapshot.CherryPicking).Clicked() {
					d.prepareAmend()
				}
				commitLabel := d.text("commit.submit")
				if d.stagedCount > 0 {
					commitLabel = fmt.Sprintf(d.text("commit.submitCount"), d.stagedCount)
				}
				commit := native.PrimarySplitButton(c, "")
				commit.Button.Label(commitLabel).Gap(detailSpacing).AlignItems(native.Center).Children(func() {
					if d.operation == "commit.running" {
						activityIcon(c)
					} else {
						native.Icon(c, icons["commit"]).Size(14, 14)
					}
					native.Text(c, commitLabel).SingleLine()
				})
				commit.Group.Height(controlHeight(c)).Disabled(d.busy || d.snapshot.CherryPicking || !canCommit || strings.TrimSpace(d.message) == "")
				commit.Arrow.Label(d.text("commit.actions")).PopoverMenuButton(func(menu *native.Menu) {
					if menu.Item(d.text("commit.push")).Chosen() {
						d.commitChanges(true)
					}
				})
				if commit.Button.Clicked() {
					d.commitChanges(false)
				}
			})
		})
	}
	diffCard := func() {
		native.Column(c).Grow(1).MinHeight(0).FillWidth().Padding(panelSpacing).Gap(controlSpacing).Background(t.Background).Radius(8).Children(func() {
			noDiff := d.comparison == nil && (d.detail == "" || d.detail == d.text("diff.empty"))
			hasSelectedFile := d.row >= 0 && d.row < len(files)
			native.Row(c).Gap(controlSpacing).AlignItems(native.Center).Hidden(noDiff && !hasSelectedFile && !d.binary).Children(func() {
				if !hasSelectedFile && (d.binary || noDiff) {
					native.Box(c).Grow(1)
				} else {
					native.Text(c, func() string {
						if d.row >= 0 && d.row < len(files) {
							return files[d.row].Path
						}
						return d.text("diff.all")
					}()).Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
				}
				if d.row >= 0 && d.row < len(files) {
					file := files[d.row]
					d.restoreFileButton(c, file)
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
					if IconButton(c, d.copy("区块操作", "Hunk actions"), "diff", false).Disabled(d.busy).Clicked() {
						d.loadHunks(files[d.row].Path)
					}
				}
			})
			if hasSelectedFile && d.comparisonLimited {
				emptyState(c, emptyDiffArt, 128, d.text("diff.previewLimited")).Grow(1)
			} else if hasSelectedFile && d.comparisonError != "" {
				emptyState(c, emptyDiffArt, 128, d.text("diff.previewFailed")).Tooltip(d.comparisonError).Grow(1)
			} else if d.binary {
				emptyState(c, emptyDiffArt, 128, d.text("diff.binary")).Grow(1)
			} else if d.comparison != nil {
				d.showCodeDiff(c, d.comparison, available-fileWidth, true)
			} else if d.detail == "" || d.detail == d.text("diff.empty") {
				if d.showWorkspaceEmptyState() {
					emptyState(c, emptyDiffArt, 128, d.text("diff.empty")).Grow(1)
				}
			} else {
				if d.workspacePatch == nil || d.workspacePatch.Buffer.String() != d.detail {
					d.workspacePatch = codePreview("changes.diff", d.detail)
				}
				editor.View(c, d.workspacePatch).Grow(1).FillWidth()
			}
		})
	}
	diffPane := func() {
		native.Column(c).Fill().MinWidth(0).Children(func() {
			d.showChangeTabs(c)
			diffCard()
		})
	}
	if splitFilePanes(c, t, &fileWidth, defaultFilePaneWidth, filesPane, diffPane) {
		d.filePane, d.filePaneChanged = filePaneWidth(available, fileWidth), true
	}
}
func (d *Desktop) toolbarStatus(c *native.Context) {
	t := c.Theme()
	native.Row(c).Grow(1).MinWidth(0).MinHeight(controlHeight(c)).Gap(detailSpacing).AlignItems(native.Center).Children(func() {
		if d.pushFailure != "" && d.pushRetryRepo == d.selected {
			native.Text(c, d.pushFailure).Grow(1).MinWidth(0).SingleLine().Ellipsis("…").Tooltip(d.pushFailure).TextColor(t.Danger)
			label := d.text("remote.push.retry")
			if IconButton(c, "", "push", false).Label(label).Tooltip(label).Disabled(d.busy).Clicked() {
				d.remoteAction("push")
			}
			return
		}
		message, color := d.remoteNotice, t.TextMuted
		switch {
		case d.err != "" && !d.cloneOpen:
			message, color = d.text("error")+": "+d.err, t.Danger
		}
		if message != "" {
			native.Text(c, message).Grow(1).MinWidth(0).SingleLine().Ellipsis("…").Tooltip(message).TextColor(color)
		}
		if d.remoteDiverged && IconButton(c, d.copy("比较上游", "Compare upstream"), "diff", false).Disabled(d.busy).Clicked() {
			d.compareUpstream()
		}
	})
}

func (d *Desktop) view(c *native.Context) {
	if d.page != projectFilesPage {
		d.cancelProjectPreview()
	}
	if d.action == "" && !d.settingsOpen && !d.repositoriesOpen && !d.cloneOpen && c.Shortcut(native.Cmd, native.KeyO) {
		d.chooseRepository()
	}
	if d.action == "" && !d.settingsOpen && !d.cloneOpen && !d.busy && c.Shortcut(native.Cmd, native.KeyK) {
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
	bar := c.TitleBar()
	if bar.Height > 0 && (runtime.GOOS == "windows" || runtime.GOOS == "darwin" && d.action != "") {
		native.Column(c).Fill().Background(t.Surface).Children(func() {
			native.Row(c).FillWidth().Height(bar.Height).Shrink(0).AlignItems(native.Center).
				Padding(0, bar.Right+panelSpacing, 0, bar.Left+panelSpacing).DragWindow().Children(func() {
				native.Text(c, d.actionTitle()).MinWidth(0).SingleLine().Ellipsis("…")
			})
			native.Box(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
				d.windowContent(c, t)
			})
		})
	} else {
		d.windowContent(c, t)
	}
	d.dialogs(c)
}

func (d *Desktop) windowContent(c *native.Context, t *native.Theme) {
	if d.action != "" {
		d.actionView(c, t)
		return
	}
	native.Column(c).Fill().Padding(panelSpacing).Gap(panelSpacing).Background(t.Surface).Children(func() {
		toolbar := native.Row(c).FillWidth().Gap(controlSpacing).AlignItems(native.Center)
		if runtime.GOOS == "darwin" {
			bar := c.TitleBar()
			toolbar.MinHeight(32).Padding(0, 0, 0, max(0, bar.Left+controlSpacing-panelSpacing)).DragWindow()
		}
		toolbar.Children(func() {
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
			d.toolbarStatus(c)
			if d.selected != "" {
				for _, op := range []struct{ label, action string }{{"remote.fetch", "fetch"}, {"remote.pull", "pull"}, {"remote.push", "push"}} {
					label := d.text(op.label)
					if activityButton(c, label, op.label, false, d.remoteRunning == op.action).Tooltip(d.text(op.label + ".hint")).Disabled(d.busy).Clicked() {
						d.remoteAction(op.action)
					}
				}
			}
			if d.refreshButton(c).Disabled(d.busy || (d.selected == "" && d.page != 4)).Clicked() {
				d.refresh()
			}
			d.settingsToolbarButton(c)
		})
		native.Row(c).Grow(1).FillWidth().Gap(panelSpacing).Children(func() {
			native.Column(c).Width(sidebarWidth).FillHeight().Gap(detailSpacing).Children(func() {
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
						d.page = item.page
						d.refresh()
					}
				}
				native.Box(c).Grow(1)
				if IconButton(c, "", "open", false).Label(d.text("repo.open")).Tooltip(d.text("repo.open")).FillWidth().Disabled(d.busy).Clicked() {
					d.chooseRepository()
				}
				if IconButton(c, "", "clone", d.cloneOpen).Label(d.text("clone.run")).Tooltip(d.text("clone.run")).FillWidth().Disabled(d.busy).Clicked() {
					d.cloneOpen = true
					d.err = ""
				}
			})
			native.Column(c).Grow(1).FillHeight().Gap(controlSpacing).Children(func() {
				if d.selected == "" && d.page != 4 {
					native.Column(c).Grow(1).FillWidth().Center().Gap(16).Children(func() {
						native.Text(c, "MyGittu").FontSize(28).Bold()
						if !d.busy && d.err == "" {
							emptyState(c, emptyPreviewArt, 128, d.text("repo.drop"))
						}
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
					d.showHistory(c, t)
				case 3:
					d.repositoryToolButtons(c)
					d.showBranches(c)
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
}

func (d *Desktop) dialogs(c *native.Context) {
	d.showProjectContentSearch(c)
	d.showStashPreview(c)
	d.cloneDialog(c)
	d.settingsDialog(c)
	d.showRepositoryTools(c)
	d.repositoryDialog(c)
	d.hunkDialog(c)
	d.showConflict(c)
	d.showFileHistory(c)
	d.showRecovery(c)
}
