package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mygittu/internal/app"
	"mygittu/internal/buildinfo"
	"mygittu/internal/settings"
	"mygittu/internal/status"
	"net"
	"sync/atomic"
	"time"
)

type Params struct {
	RepoID   string          `json:"repo_id"`
	Path     string          `json:"path"`
	Paths    []string        `json:"paths"`
	Message  string          `json:"message"`
	Action   string          `json:"action"`
	Name     string          `json:"name"`
	Staged   bool            `json:"staged"`
	Limit    int             `json:"limit"`
	Offset   int             `json:"offset"`
	Query    string          `json:"query"`
	Hash     string          `json:"hash"`
	Settings settings.Values `json:"settings"`
	Identity app.Identity    `json:"identity"`
	Scope    string          `json:"scope"`
}
type Server struct {
	Service *app.Service
	Cache   *status.Cache
}

func (s *Server) Handle(ctx context.Context, r Request) Response {
	resp := Response{Version: Version, ID: r.ID}
	fail := func(code, msg string) Response {
		resp.OK = false
		resp.Result = nil
		resp.Error = &Failure{code, msg}
		return resp
	}
	if r.Version != Version {
		return fail("version", "unsupported protocol version")
	}
	if r.ID == "" {
		return fail("request", "request_id required")
	}
	var p Params
	if len(r.Params) > 0 {
		if e := json.Unmarshal(r.Params, &p); e != nil {
			return fail("params", "invalid parameters")
		}
	}
	var result any
	var e error
	switch r.Method {
	case "health.ping":
		result = map[string]any{"version": Version, "app_version": buildinfo.Version}
	case "settings.get":
		if s.Service.Settings == nil {
			result = settings.Values{Language: "zh-CN", Theme: "system"}
		} else {
			result = s.Service.Settings.Get()
		}
	case "settings.update":
		if s.Service.Settings == nil {
			e = fmt.Errorf("settings store unavailable")
		} else {
			e = s.Service.Settings.Update(p.Settings)
			result = s.Service.Settings.Get()
		}
	case "repo.list":
		result = s.Service.Registry.List()
	case "identity.get":
		result, e = s.Service.Identity(ctx, p.RepoID, p.Scope)
	case "identity.update":
		result, e = s.Service.UpdateIdentity(ctx, p.RepoID, p.Identity)
	case "repo.register":
		result, e = s.Service.Registry.Register(ctx, p.Path)
	case "repo.unregister":
		e = s.Service.Registry.Unregister(p.RepoID)
		s.Cache.Invalidate(p.RepoID)
	case "overlay.lookup":
		result = s.Cache.Lookup(p.Path)
	case "ui.open":
		e = s.Service.OpenDesktop(ctx, p.Path, p.Action)
	case "status.refresh":
		for attempt := 0; attempt < 3; attempt++ {
			e = s.Cache.Refresh(ctx, p.RepoID)
			if !errors.Is(e, status.ErrStale) {
				break
			}
		}
		if e == nil {
			var ok bool
			result, ok = s.Cache.Get(p.RepoID)
			if !ok {
				e = status.ErrStale
			}
		}
	case "status.get":
		var ok bool
		result, ok = s.Cache.Get(p.RepoID)
		if !ok {
			return fail("cache_miss", "status is refreshing")
		}
	case "git.stage", "git.unstage":
		e = s.Service.Paths(ctx, p.RepoID, p.Paths, r.Method == "git.stage")
	case "git.commit":
		result, e = s.Service.Commit(ctx, p.RepoID, p.Message)
	case "git.log":
		if p.Limit == 0 {
			p.Limit = 100
		}
		result, e = s.Service.LogSearch(ctx, p.RepoID, p.Limit, p.Offset, p.Query)
	case "git.show":
		result, e = s.Service.Show(ctx, p.RepoID, p.Hash)
	case "git.diff":
		result, e = s.Service.Diff(ctx, p.RepoID, p.Path, p.Staged)
	case "git.branches":
		result, e = s.Service.Branches(ctx, p.RepoID)
	case "git.branch":
		e = s.Service.Branch(ctx, p.RepoID, p.Action, p.Name)
	case "git.remote":
		e = s.Service.Remote(ctx, p.RepoID, p.Action)
	default:
		return fail("method", "unsupported method")
	}
	if r.Method == "git.stage" || r.Method == "git.unstage" || r.Method == "git.commit" || r.Method == "git.branch" || r.Method == "git.remote" {
		s.Cache.Invalidate(p.RepoID)
	}
	if e != nil {
		return fail("operation", e.Error())
	}
	resp.OK = true
	resp.Result, e = json.Marshal(result)
	if e != nil || len(resp.Result) > MaxFrame-1024 {
		return fail("size", "result too large; narrow the request")
	}
	return resp
}
func (s *Server) Serve(ctx context.Context, l net.Listener) error {
	defer l.Close()
	go func() { <-ctx.Done(); l.Close() }()
	sem := make(chan struct{}, 32)
	for {
		c, e := l.Accept()
		if e != nil {
			if ctx.Err() != nil {
				return nil
			}
			return e
		}
		select {
		case sem <- struct{}{}:
			go func() {
				defer func() { <-sem }()
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(3 * time.Minute))
				var req Request
				if ReadFrame(c, &req) != nil {
					return
				}
				timeout := 2 * time.Minute
				if req.Method == "overlay.lookup" {
					timeout = 40 * time.Millisecond
				}
				task, cancel := context.WithTimeout(ctx, timeout)
				defer cancel()
				_ = WriteFrame(c, s.Handle(task, req))
			}()
		default:
			c.Close()
		}
	}
}

type Dialer func(context.Context) (net.Conn, error)
type Client struct {
	Dial Dialer
	next atomic.Uint64
}

func (c *Client) Call(ctx context.Context, method string, p Params, out any) error {
	conn, e := c.Dial(ctx)
	if e != nil {
		return e
	}
	defer conn.Close()
	deadline := time.Now().Add(2 * time.Minute)
	if d, ok := ctx.Deadline(); ok {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-done:
		}
	}()
	b, _ := json.Marshal(p)
	req := Request{Version, fmt.Sprint(c.next.Add(1)), method, b}
	if e = WriteFrame(conn, req); e != nil {
		return e
	}
	var r Response
	if e = ReadFrame(conn, &r); e != nil {
		return e
	}
	if r.Version != Version || r.ID != req.ID {
		return fmt.Errorf("invalid response")
	}
	if !r.OK {
		if r.Error == nil {
			return fmt.Errorf("invalid error response")
		}
		return fmt.Errorf("%s", r.Error.Message)
	}
	if out != nil {
		return json.Unmarshal(r.Result, out)
	}
	return nil
}
