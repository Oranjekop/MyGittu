package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
	native "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"mygittu/internal/app"
	"mygittu/internal/ipc"
)

type projectContentSearch struct {
	open       bool
	query, err string
	request    *projectSearchRequest
	found      bool
	result     app.FileSearch
	list       native.ListState
	row        int
	jump       app.FileMatch
}

type projectSearchRequest struct{ cancel context.CancelFunc }

func (s *projectContentSearch) cancelRequest() {
	if s.request != nil {
		s.request.cancel()
		s.request = nil
	}
}

func (d *Desktop) searchProjectContent() {
	s := &d.fileSearch
	s.cancelRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	request := &projectSearchRequest{cancel: cancel}
	s.request = request
	params := ipc.Params{RepoID: d.selected, Query: strings.TrimSpace(s.query)}
	s.found, s.err = false, ""
	go func() {
		defer cancel()
		var result app.FileSearch
		err := d.Client.Call(ctx, "git.files.search", params, &result)
		mygo.RunOnMain(func() {
			if s.request != request || !s.open || d.selected != params.RepoID || d.win != nil && d.win.IsDestroyed() {
				return
			}
			s.request = nil
			if err == nil {
				s.result, s.found, s.row = result, true, -1
				s.list = native.ListState{}
			} else {
				s.err = err.Error()
			}
			if d.win != nil {
				d.win.Invalidate()
			}
		})
	}()
}

func (d *Desktop) showProjectContentSearch(c *native.Context) {
	s := &d.fileSearch
	width, height := c.Size()
	native.Modal(c, &s.open, func() {
		native.Column(c).Width(min(780, width-72)).Gap(controlSpacing).Children(func() {
			native.Row(c).FillWidth().Gap(controlSpacing).AlignItems(native.Center).Children(func() {
				before := s.query
				native.TextInput(c, &s.query).Grow(1).MinWidth(0).Placeholder(d.copy("搜索文件内容", "Search file contents"))
				if before != s.query {
					s.cancelRequest()
					s.found, s.err = false, ""
				}
				if IconButton(c, d.text("search"), "search", true).Disabled(d.Client == nil || strings.TrimSpace(s.query) == "").Clicked() {
					d.searchProjectContent()
				}
				if IconButton(c, d.text("close"), "close", false).Clicked() {
					s.open = false
				}
			})
			switch {
			case s.request != nil:
				native.Text(c, d.text("busy"))
			case s.err != "":
				native.Text(c, s.err).TextColor(c.Theme().Danger)
			case s.found && len(s.result.Matches) == 0 && !s.result.Limited:
				native.Text(c, d.copy("没有匹配内容", "No matches"))
			case s.found:
				if s.result.Limited {
					native.Text(c, d.copy("结果已截断", "Results truncated"))
				}
				previous := s.row
				s.list.Selected = &s.row
				roundedTable(c, &s.list, []native.TableColumn{{Title: d.text("file"), Width: 240}, {Title: d.copy("命中内容", "Match")}}, len(s.result.Matches), func(row, col int) {
					match := s.result.Matches[row]
					value := match.Text
					if col == 0 {
						value = fmt.Sprintf("%s:%d", match.Path, match.Line)
					}
					native.Text(c, value).SingleLine().Ellipsis("…").Tooltip(value)
				}).Height(min(400, height-170))
				if s.row != previous && s.row >= 0 && s.row < len(s.result.Matches) {
					s.open, s.jump = false, s.result.Matches[s.row]
					d.openProjectTab(s.jump.Path)
					d.jumpToProjectMatch()
				}
			}
		})
	})
	if !s.open {
		s.cancelRequest()
	}
}

func (d *Desktop) jumpToProjectMatch() {
	jump := d.fileSearch.jump
	if jump.Path != "" && d.projectShownPath == jump.Path && d.projectCode != nil {
		d.projectCode.GoTo(editor.Pos{Line: jump.Line - 1})
		d.fileSearch.jump = app.FileMatch{}
	}
}
