package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mygittu/internal/app"
	"mygittu/internal/buildinfo"
	"mygittu/internal/commitai"
	git "mygittu/internal/git"
	"mygittu/internal/repository"
	"mygittu/internal/settings"
	"mygittu/internal/status"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type Params struct {
	AllBranches    bool            `json:"all_branches,omitempty"`
	FileContent    []byte          `json:"file_content,omitempty"`
	PreviewOffset  int64           `json:"preview_offset,omitempty"`
	PreviewVersion string          `json:"preview_version,omitempty"`
	CloneAuth      git.CloneAuth   `json:"clone_auth,omitempty"`
	RepoID         string          `json:"repo_id"`
	Path           string          `json:"path"`
	Paths          []string        `json:"paths"`
	Message        string          `json:"message"`
	Action         string          `json:"action"`
	Name           string          `json:"name"`
	Staged         bool            `json:"staged"`
	Limit          int             `json:"limit"`
	Offset         int             `json:"offset"`
	Query          string          `json:"query"`
	Hash           string          `json:"hash"`
	URL            string          `json:"url"`
	Destination    string          `json:"destination"`
	OriginalPath   string          `json:"original_path"`
	StatusUpdated  time.Time       `json:"status_updated,omitempty"`
	Settings       settings.Values `json:"settings"`
	Identity       app.Identity    `json:"identity"`
	Scope          string          `json:"scope"`
}
type Server struct {
	Service   *app.Service
	Cache     *status.Cache
	discovery sync.Map
}

func (s *Server) ensureRepository(path string) bool {
	if _, ok := s.Service.Registry.FindPath(path); ok {
		return true
	}
	root, ok := repository.FindGitRoot(path)
	if !ok {
		return false
	}
	key := repository.Key(root)
	if _, loaded := s.discovery.LoadOrStore(key, struct{}{}); !loaded {
		go func() {
			defer s.discovery.Delete(key)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			info, err := s.Service.Registry.RegisterDiscovered(ctx, root)
			if err != nil || s.Cache == nil {
				return
			}
			_ = s.Cache.Refresh(ctx, info.ID)
		}()
	}
	return true
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
	case "update.check":
		result, e = s.Service.CheckUpdate(ctx)
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
	case "repo.contains":
		ok := filepath.IsAbs(p.Path) && s.ensureRepository(p.Path)
		result = map[string]any{"repository": ok}
	case "repo.unregister":
		e = s.Service.Registry.Unregister(p.RepoID)
		s.Cache.Invalidate(p.RepoID)
	case "overlay.lookup":
		if s.Cache.Lookup(p.Path).State == git.Unknown && filepath.IsAbs(p.Path) {
			s.ensureRepository(p.Path)
		}
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
	case "status.poll":
		result = s.Cache.GetSince(p.RepoID, p.StatusUpdated)
	case "git.stage", "git.unstage":
		e = s.Service.Paths(ctx, p.RepoID, p.Paths, r.Method == "git.stage")
	case "git.commit":
		result, e = s.Service.Commit(ctx, p.RepoID, p.Message)
	case "git.log":
		if p.Limit == 0 {
			p.Limit = 100
		}
		result, e = s.Service.History(ctx, p.RepoID, p.Limit, p.Offset, p.Query, p.AllBranches)
	case "git.show":
		result, e = s.Service.Show(ctx, p.RepoID, p.Hash)
	case "git.commit.details":
		result, e = s.Service.CommitDetails(ctx, p.RepoID, p.Hash)
	case "git.hunks":
		result, e = s.Service.StageHunks(ctx, p.RepoID, p.Path)
	case "git.hunk.stage":
		e = s.Service.StageSelectedHunk(ctx, p.RepoID, p.Path, p.Hash, p.Offset)
	case "git.references":
		result, e = s.Service.References(ctx, p.RepoID)
	case "git.conflict":
		result, e = s.Service.Conflict(ctx, p.RepoID, p.Path)
	case "git.resolve":
		e = s.Service.ResolveConflict(ctx, p.RepoID, p.Path, p.Hash, p.Message)
	case "git.file.log":
		result, e = s.Service.FileLog(ctx, p.RepoID, p.Path, p.Limit, p.Offset)
	case "git.file.insight":
		result, e = s.Service.FileInsight(ctx, p.RepoID, p.Path, p.Action, p.Hash)
	case "git.file.blame":
		result, e = s.Service.FileBlame(ctx, p.RepoID, p.Path)
	case "git.remotes":
		result, e = s.Service.Remotes(ctx, p.RepoID)
	case "git.remote.configure":
		e = s.Service.ConfigureRemote(ctx, p.RepoID, p.Action, p.Name, p.URL)
	case "git.amend.target":
		result, e = s.Service.AmendTarget(ctx, p.RepoID)
	case "git.amend":
		e = s.Service.Amend(ctx, p.RepoID, p.Hash, p.Message)
	case "git.cherry-pick":
		e = s.Service.CherryPick(ctx, p.RepoID, p.Action, p.Hash)
	case "git.worktrees":
		result, e = s.Service.Worktrees(ctx, p.RepoID)
	case "git.worktree.edit":
		e = s.Service.EditWorktree(ctx, p.RepoID, p.Action, p.Path, p.Name)
	case "git.restore.preview":
		result, e = s.Service.RestoreToken(ctx, p.RepoID, p.Path, p.Action == "untracked")
	case "git.restore":
		e = s.Service.RestoreFile(ctx, p.RepoID, p.Path, p.Hash, p.Action == "untracked")
	case "git.hunk.restore":
		e = s.Service.RestoreSelectedHunk(ctx, p.RepoID, p.Path, p.Hash, p.Offset)
	case "git.recover":
		e = s.Service.RecoverCommit(ctx, p.RepoID, p.Action, p.Hash)
	case "git.compare":
		result, e = s.Service.CompareBranches(ctx, p.RepoID, p.Name, p.Query)
	case "git.compare.targets":
		result, e = s.Service.CompareTargets(ctx, p.RepoID)
	case "git.branch.entries":
		result, e = s.Service.BranchEntries(ctx, p.RepoID)
	case "git.reference":
		e = s.Service.EditReference(ctx, p.RepoID, p.Action, p.Name)
	case "git.files":
		result, e = s.Service.Files(ctx, p.RepoID)
	case "git.file.preview":
		result, e = s.Service.PreviewFile(ctx, p.RepoID, p.Path, p.PreviewOffset, p.PreviewVersion)
	case "git.file.save":
		result, e = s.Service.SaveProjectFile(ctx, p.RepoID, p.Path, p.PreviewVersion, p.FileContent)
		if e == nil && s.Cache != nil {
			s.Cache.ScheduleRefresh(p.RepoID)
		}
	case "git.diff.staged-for-message":
		var patch string
		patch, e = s.Service.Diff(ctx, p.RepoID, "", true)
		if e == nil {
			if len(patch) > commitai.MaxPatch {
				e = fmt.Errorf("staged diff exceeds 48 KiB")
			} else {
				result = patch
			}
		}
	case "git.diff":
		result, e = s.Service.Diff(ctx, p.RepoID, p.Path, p.Staged)
	case "git.comparison":
		result, e = s.Service.Comparison(ctx, p.RepoID, p.Path, p.OriginalPath, p.Staged)
	case "git.branches":
		result, e = s.Service.Branches(ctx, p.RepoID)
	case "git.branch":
		e = s.Service.Branch(ctx, p.RepoID, p.Action, p.Name)
	case "git.remote":
		result, e = s.Service.RemoteWithResult(ctx, p.RepoID, p.Action)
	case "git.github.authorize":
		e = s.Service.Runner.AuthorizeGitHub(ctx, p.Name)
	case "git.github.repositories":
		result, e = s.Service.Runner.GitHubRepositories(ctx, p.Name)
	case "git.clone":
		result, e = s.Service.Clone(ctx, p.URL, p.Destination, p.CloneAuth)
	default:
		return fail("method", "unsupported method")
	}
	switch r.Method {
	case "git.stage", "git.unstage", "git.commit", "git.branch", "git.remote", "git.amend", "git.cherry-pick", "git.remote.configure", "git.restore", "git.hunk.restore", "git.hunk.stage", "git.recover", "git.resolve", "git.worktree.edit":
		if s.Cache != nil {
			s.Cache.ScheduleRefresh(p.RepoID)
		}
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
