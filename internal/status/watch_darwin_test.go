package status

import (
	"context"
	git "mygittu/internal/git"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRefreshCompletesWhenFileChangesDuringScan(t *testing.T) {
	cache, repo := fixture(t)
	dir := t.TempDir()
	t.Setenv("MYGITTU_TEST_GIT", cache.Service.Runner.Binary)
	t.Setenv("MYGITTU_TEST_SCAN", filepath.Join(dir, "scan"))
	wrapper := filepath.Join(dir, "git")
	if err := os.WriteFile(wrapper, []byte(`#!/bin/sh
set -e
if [ "$3" = status ] && [ -f "$MYGITTU_TEST_SCAN" ]; then
	rm "$MYGITTU_TEST_SCAN"
	"$MYGITTU_TEST_GIT" "$@" > "$MYGITTU_TEST_SCAN.out"
	touch changed-during-scan
	sleep 1
	cat "$MYGITTU_TEST_SCAN.out"
else
	exec "$MYGITTU_TEST_GIT" "$@"
fi
`), 0700); err != nil {
		t.Fatal(err)
	}
	runner, err := git.NewRunner(wrapper)
	if err != nil {
		t.Fatal(err)
	}
	cache.Service.Runner = runner

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	done := make(chan struct{})
	go func() { cache.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()

	await := func(state git.State) {
		t.Helper()
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for cache.Lookup(repo.Root).State != state {
			select {
			case <-ctx.Done():
				t.Fatalf("expected %s, got %+v", state, cache.Lookup(repo.Root))
			case <-ticker.C:
			}
		}
	}
	await(git.Clean)
	if err := os.WriteFile(filepath.Join(dir, "scan"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := cache.Refresh(ctx, repo.ID); err != nil {
		t.Fatal(err)
	}
	await(git.Untracked)
}
