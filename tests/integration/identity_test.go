package integration

import (
	"context"
	"mygittu/internal/app"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIdentityScopeAndCommit(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "global.gitconfig"))
	s, id, dir := setup(t)
	ctx := context.Background()
	v, e := s.UpdateIdentity(ctx, id, app.Identity{Name: "全局用户", Email: "global@example.invalid", Scope: "global"})
	if e != nil || v.Name != "全局用户" {
		t.Fatalf("global identity: %+v %v", v, e)
	}
	local, e := s.Identity(ctx, id, "local")
	if e != nil || local.Name != "Test" {
		t.Fatal("global update overwrote local identity", e)
	}
	_, e = s.UpdateIdentity(ctx, id, app.Identity{Name: "项目作者", Email: "project@example.invalid", Scope: "local"})
	if e != nil {
		t.Fatal(e)
	}
	global, e := s.Identity(ctx, id, "global")
	if e != nil || global.Email != "global@example.invalid" {
		t.Fatal("local update overwrote global default")
	}
	write(t, filepath.Join(dir, "a"), "a")
	if e = s.Paths(ctx, id, []string{"a"}, true); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Commit(ctx, id, "identity test"); e != nil {
		t.Fatal(e)
	}
	actual := command(t, dir, "log", "-1", "--format=%an <%ae>|%cn <%ce>")
	if !strings.Contains(actual, "项目作者 <project@example.invalid>|项目作者 <project@example.invalid>") {
		t.Fatal(actual)
	}
	for _, bad := range []app.Identity{{Name: "bad\nname", Email: "x@example.invalid", Scope: "local"}, {Name: "Name", Email: "invalid", Scope: "local"}, {Name: "Name", Email: "x@example.invalid", Scope: "system"}} {
		if _, e = s.UpdateIdentity(ctx, id, bad); e == nil {
			t.Fatalf("accepted invalid identity: %+v", bad)
		}
	}
}
func TestIdentityLockedConfigUnchanged(t *testing.T) {
	s, id, dir := setup(t)
	path := filepath.Join(dir, ".git", "config")
	original, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path+".lock", []byte("another Git process"), 0600); e != nil {
		t.Fatal(e)
	}
	_, e = s.UpdateIdentity(context.Background(), id, app.Identity{Name: "Should not change", Email: "no@example.invalid", Scope: "local"})
	if e == nil {
		t.Fatal("ignored existing Git lock")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(original) {
		t.Fatal("config changed despite lock failure")
	}
}
