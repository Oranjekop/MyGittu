package ui

import (
	"context"
	"github.com/egoist/mygo"
	"mygittu/internal/app"
	"mygittu/internal/ipc"
	"mygittu/internal/repository"
)

func (d *Desktop) openWork(ctx context.Context, path string) (func(), error) {
	var info repository.Info
	if e := d.Client.Call(ctx, "repo.register", ipc.Params{Path: path}, &info); e != nil {
		return nil, e
	}
	var repos []repository.Info
	e := d.Client.Call(ctx, "repo.list", ipc.Params{}, &repos)
	return func() {
		d.repos = repos
		d.selectRepository(info.ID, info.Root)
		d.offset = 0
		if d.page == 6 {
			d.page = 0
		}
		d.refresh()
	}, e
}
func (d *Desktop) open(path string) {
	if path == "" || d.busy {
		return
	}
	d.task(func(ctx context.Context) (func(), error) { return d.openWork(ctx, path) })
}
func (d *Desktop) chooseRepository() {
	if d.busy {
		return
	}
	title, initial := d.text("repo.dialog"), d.path
	d.task(func(ctx context.Context) (func(), error) {
		paths, e := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: d.win, Directory: true, Title: title, DefaultPath: initial})
		if e != nil || len(paths) == 0 {
			return nil, e
		}
		return d.openWork(ctx, paths[0])
	})
}
func (d *Desktop) loadIdentity() {
	scope := "local"
	if d.identityScope == 1 || d.selected == "" {
		scope = "global"
	}
	id := d.selected
	d.task(func(ctx context.Context) (func(), error) {
		var v app.Identity
		e := d.Client.Call(ctx, "identity.get", ipc.Params{RepoID: id, Scope: scope}, &v)
		return func() { d.identity = editableIdentity(v); d.identitySaved = false }, e
	})
}
func editableIdentity(v app.Identity) app.Identity {
	if v.Name == "" {
		v.Name = v.EffectiveName
	}
	if v.Email == "" {
		v.Email = v.EffectiveEmail
	}
	return v
}
func (d *Desktop) saveIdentity() {
	id, v := d.selected, d.identity
	d.task(func(ctx context.Context) (func(), error) {
		var saved app.Identity
		e := d.Client.Call(ctx, "identity.update", ipc.Params{RepoID: id, Identity: v}, &saved)
		var current app.Identity
		if e == nil && id != "" {
			e = d.Client.Call(ctx, "identity.get", ipc.Params{RepoID: id, Scope: "local"}, &current)
		}
		return func() { d.identity = saved; d.identitySaved = true; d.commitIdentity = current }, e
	})
}
