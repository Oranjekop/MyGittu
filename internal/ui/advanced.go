package ui

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/egoist/mygo"
	native "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"mygittu/internal/app"
	"mygittu/internal/ipc"
)

type advancedPanel struct {
	open, staged                  bool
	mode, base, good, bad, branch string
	patch, output, err            string
	check                         app.PatchCheck
	preview                       patchView
	state                         app.AdvancedState
	steps                         []app.RebaseStep
	logs                          []app.ReflogEntry
	row                           int
	list                          native.ListState
}

func (d *Desktop) advancedAction(mode string, selected app.Commit) {
	p := &d.advanced
	switch mode {
	case "compare":
		if d.compareCommit == "" {
			d.compareCommit = selected.Hash
			return
		}
		base, id := d.compareCommit, d.selected
		d.task(func(ctx context.Context) (func(), error) {
			var result app.BranchComparisonSummary
			err := d.Client.Call(ctx, "git.compare.commits", ipc.Params{RepoID: id, Name: base, Query: selected.Hash}, &result)
			return func() {
				d.tools.setComparison(result)
				d.compareCommit = ""
				*p = advancedPanel{open: true, mode: mode}
			}, err
		})
	case "reflog":
		id := d.selected
		d.task(func(ctx context.Context) (func(), error) {
			var entries []app.ReflogEntry
			err := d.Client.Call(ctx, "git.reflog", ipc.Params{RepoID: id}, &entries)
			return func() { *p = advancedPanel{open: true, mode: mode, logs: entries} }, err
		})
	case "rebase", "bisect":
		id := d.selected
		d.task(func(ctx context.Context) (func(), error) {
			var state app.AdvancedState
			err := d.Client.Call(ctx, "git.advanced.state", ipc.Params{RepoID: id}, &state)
			return func() {
				*p = advancedPanel{open: true, mode: mode, state: state, base: selected.Hash, good: selected.Hash}
			}, err
		})
	default:
		*p = advancedPanel{open: true, mode: mode}
	}
}

func (d *Desktop) showAdvancedDialog(c *native.Context) {
	if d.action != "" {
		return
	}
	width, height := c.Size()
	native.Modal(c, &d.advanced.open, func() {
		native.Column(c).Width(min(820, width-72)).Height(min(520, height-100)).Gap(panelSpacing).Children(func() {
			d.advancedContent(c)
		})
	})
}

func (d *Desktop) advancedContent(c *native.Context) {
	p := &d.advanced
	native.Row(c).FillWidth().Gap(controlSpacing).Children(func() {
		native.Text(c, d.text("advanced."+p.mode)).Bold().Grow(1)
		if IconButton(c, d.text("close"), "close", false).Disabled(d.busy).Clicked() {
			if d.action == "file-patch" && d.win != nil {
				d.win.Close()
			} else {
				p.open = false
			}
		}
	})
	if p.err != "" {
		native.Text(c, p.err).FillWidth().TextColor(c.Theme().Danger)
	}
	switch p.mode {
	case "compare":
		d.showComparisonFiles(c)
	case "reflog":
		if len(p.logs) == 0 {
			emptyState(c, emptyPreviewArt, 128, d.text("advanced.emptyReflog")).Grow(1)
			break
		}
		p.list.Selected = &p.row
		roundedTable(c, &p.list, []native.TableColumn{{Title: "HEAD", Width: 120}, {Title: d.text("subject")}}, len(p.logs), func(row, col int) {
			entry := p.logs[row]
			native.Text(c, []string{entry.Selector, entry.Summary}[col]).SingleLine().Ellipsis("…").Tooltip(entry.Hash)
		}).Grow(1).FillWidth()
		native.Row(c).Gap(controlSpacing).Children(func() {
			native.TextInput(c, &p.branch).Placeholder(d.text("branch.name")).Grow(1)
			if IconButton(c, d.text("branch.create"), "branch", true).Disabled(d.busy || p.branch == "" || p.row < 0 || p.row >= len(p.logs)).Clicked() {
				d.runAdvanced("git.branch.at", ipc.Params{Name: strings.TrimSpace(p.branch), Hash: p.logs[p.row].Hash})
			}
		})
	case "patch":
		d.patchControls(c)
	case "rebase":
		d.rebaseControls(c)
	case "bisect":
		d.bisectControls(c)
	}
	if len(p.preview.lines) > 0 {
		native.Text(c, p.output).TextColor(c.Theme().TextMuted).SingleLine().Ellipsis("…").Tooltip(p.output)
		d.showPatch(c, &p.preview).Grow(1).FillWidth()
	} else if p.output != "" {
		scrollAtPanelEdge(native.Scroll(c)).Grow(1).MinHeight(0).FillWidth().Children(func() {
			native.Text(c, p.output).FillWidth()
		})
	}
}

func (d *Desktop) runAdvanced(method string, params ipc.Params) {
	p := &d.advanced
	params.RepoID = d.selected
	d.task(func(ctx context.Context) (func(), error) {
		var output string
		var dest any
		if method == "git.bisect" {
			dest = &output
		}
		err := d.Client.Call(ctx, method, params, dest)
		var state app.AdvancedState
		stateErr := d.Client.Call(ctx, "git.advanced.state", ipc.Params{RepoID: params.RepoID}, &state)
		return func() {
			if stateErr == nil {
				p.state = state
			}
			p.err = ""
			if err != nil {
				p.err = err.Error()
				if stateErr == nil && state.Rebasing {
					d.refresh()
				}
			} else if stateErr != nil {
				p.err = stateErr.Error()
			} else {
				p.output = output
				if output == "" {
					p.output = d.text("file.actionDone")
				}
				d.refresh()
			}
			if method == "git.rebase" {
				p.steps = nil
			}
		}, nil
	})
}

func (d *Desktop) rebaseControls(c *native.Context) {
	p := &d.advanced
	if p.state.Rebasing {
		d.advancedSteps(c, "git.rebase", []string{"continue", "skip", "abort"})
		if IconButton(c, d.text("advanced.resolveRebase"), "changes", false).Disabled(d.busy).Clicked() {
			p.open = false
			d.page = 0
			d.changesCollapsed = false
			d.refresh()
		}
		return
	}
	native.Row(c).Gap(controlSpacing).Children(func() {
		base := p.base
		native.TextInput(c, &p.base).Placeholder(d.text("advanced.base")).Grow(1).Disabled(d.busy)
		if p.base != base {
			p.steps = nil
		}
		if IconButton(c, d.text("advanced.plan"), "history", false).Disabled(d.busy || p.base == "").Clicked() {
			params := ipc.Params{RepoID: d.selected, Hash: strings.TrimSpace(p.base)}
			d.task(func(ctx context.Context) (func(), error) {
				var steps []app.RebaseStep
				err := d.Client.Call(ctx, "git.rebase.plan", params, &steps)
				return func() {
					p.err = ""
					if err != nil {
						p.err = err.Error()
					}
					p.steps = steps
				}, nil
			})
		}
	})
	if len(p.steps) == 0 {
		if p.err == "" && p.output == "" && !d.busy {
			emptyState(c, emptyPreviewArt, 128, d.text("advanced.emptyPlan")).Grow(1)
		}
		return
	}
	native.Text(c, d.text("advanced.rewrite")).FillWidth()
	move := -1
	direction := 0
	scrollAtPanelEdge(native.Scroll(c)).Grow(1).MinHeight(0).FillWidth().Children(func() {
		native.Column(c).FillWidth().Gap(controlSpacing).Children(func() {
			for i, step := range p.steps {
				native.Row(c.Key(i)).Gap(controlSpacing).Children(func() {
					native.Text(c, step.Hash).Grow(1).SingleLine().Ellipsis("…").Tooltip(step.Hash)
					ops := []string{"pick", "fixup", "drop"}
					index := 0
					for j, op := range ops {
						if step.Action == op {
							index = j
						}
					}
					native.Segmented(c, &index, d.text("advanced.pick"), d.text("advanced.fixup"), d.text("advanced.drop")).Disabled(d.busy)
					p.steps[i].Action = ops[index]
					if IconButton(c, "↑", "", false).Disabled(d.busy || i == 0).Clicked() {
						move, direction = i, -1
					}
					if IconButton(c, "↓", "", false).Disabled(d.busy || i+1 == len(p.steps)).Clicked() {
						move, direction = i, 1
					}
				})
			}
		})
	})
	if move >= 0 {
		p.steps[move], p.steps[move+direction] = p.steps[move+direction], p.steps[move]
	}
	if IconButton(c, d.text("advanced.start"), "branch", true).Disabled(d.busy).Clicked() {
		params := ipc.Params{Action: "start", Hash: strings.TrimSpace(p.base), RebaseSteps: append([]app.RebaseStep(nil), p.steps...)}
		d.runAdvanced("git.rebase", params)
	}
}

func (d *Desktop) advancedSteps(c *native.Context, method string, actions []string) {
	native.Row(c).Gap(controlSpacing).Wrap().Children(func() {
		for _, action := range actions {
			if IconButton(c, d.text("advanced."+action), "", action == "continue").Disabled(d.busy).Clicked() {
				d.runAdvanced(method, ipc.Params{Action: action})
			}
		}
	})
}

func (d *Desktop) bisectControls(c *native.Context) {
	p := &d.advanced
	if p.state.Bisecting {
		d.advancedSteps(c, "git.bisect", []string{"good", "bad", "skip", "reset"})
		return
	}
	native.TextInput(c, &p.good).Placeholder(d.text("advanced.goodCommit")).FillWidth().Disabled(d.busy)
	native.TextInput(c, &p.bad).Placeholder(d.text("advanced.badCommit")).FillWidth().Disabled(d.busy)
	if IconButton(c, d.text("advanced.start"), "branch", true).Disabled(d.busy || p.good == "" || p.bad == "").Clicked() {
		d.runAdvanced("git.bisect", ipc.Params{Action: "start", Hash: strings.TrimSpace(p.bad), Name: strings.TrimSpace(p.good)})
	}
	if p.err == "" && p.output == "" && !d.busy {
		emptyState(c, emptyPreviewArt, 128, d.text("advanced.emptyBisect")).Grow(1)
	}
}

func (d *Desktop) patchControls(c *native.Context) {
	p := &d.advanced
	if d.action != "file-patch" {
		native.Row(c).Gap(controlSpacing).Wrap().Children(func() {
			native.Checkbox(c, &p.staged, d.text("index"))
			if IconButton(c, d.text("advanced.export"), "save", false).Disabled(d.busy).Clicked() {
				id, staged := d.selected, p.staged
				d.task(func(ctx context.Context) (func(), error) {
					var patch []byte
					if err := d.Client.Call(ctx, "git.patch.export", ipc.Params{RepoID: id, Staged: staged}, &patch); err != nil {
						return func() { p.err = err.Error() }, nil
					}
					path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: d.win, Title: d.text("advanced.export"), DefaultPath: "changes.patch"})
					if err == nil && path != "" {
						err = os.WriteFile(path, patch, 0644)
					}
					return func() {
						if err != nil {
							p.err = err.Error()
						}
					}, nil
				})
			}
			if IconButton(c, d.text("advanced.openPatch"), "open", false).Disabled(d.busy).Clicked() {
				d.previewPatch("")
			}
		})
	}
	if p.check.Token != "" {
		if IconButton(c, d.text("advanced.apply"), "commit", true).Disabled(d.busy).Clicked() {
			params := ipc.Params{RepoID: d.selected, FileContent: []byte(p.patch), Hash: p.check.Token}
			d.task(func(ctx context.Context) (func(), error) {
				err := d.Client.Call(ctx, "git.patch.apply", params, nil)
				return func() {
					if err != nil {
						p.err = err.Error()
						return
					}
					p.check, p.patch, p.err, p.preview = app.PatchCheck{}, "", "", patchView{}
					p.output = d.text("file.actionDone")
					d.refresh()
				}, nil
			})
		}
	}
	if p.err == "" && p.output == "" && !d.busy {
		emptyState(c, emptyDiffArt, 128, d.text("advanced.emptyPatch")).Grow(1)
	}
}

func (d *Desktop) previewPatch(path string) {
	p := &d.advanced
	id := d.selected
	p.err, p.output, p.patch, p.check, p.preview = "", "", "", app.PatchCheck{}, patchView{}
	d.task(func(ctx context.Context) (func(), error) {
		var patch string
		var check app.PatchCheck
		err := func() error {
			if path == "" {
				paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: d.win, Title: d.text("advanced.openPatch")})
				if err != nil || len(paths) == 0 {
					return err
				}
				path = paths[0]
			}
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			data, err := io.ReadAll(io.LimitReader(file, app.MaxPatchBytes+1))
			if err != nil {
				return err
			}
			if len(data) > app.MaxPatchBytes {
				return fmt.Errorf("patch exceeds %d bytes", app.MaxPatchBytes)
			}
			patch = string(data)
			return d.Client.Call(ctx, "git.patch.apply", ipc.Params{RepoID: id, FileContent: data}, &check)
		}()
		files, _ := editor.ParsePatch(patch)
		return func() {
			if err != nil {
				p.err = err.Error()
				return
			}
			p.patch, p.check, p.output = patch, check, check.Summary
			p.preview = patchView{lines: d.patchRows(files, true)}
		}, nil
	})
}
