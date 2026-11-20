package integration

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStashPreviewAndContentSearch(t *testing.T) {
	s, id, dir := setup(t)
	ctx := context.Background()
	file := filepath.Join(dir, "tracked: path.txt")
	if filepath.Separator == '\\' {
		file = filepath.Join(dir, "tracked path.txt")
	}
	write(t, file, "first\nNeedle in tracked content\n"+strings.Repeat("needle\n", 205))
	command(t, dir, "add", "--", filepath.Base(file))
	command(t, dir, "commit", "-m", "base")
	write(t, filepath.Join(dir, "untracked.txt"), "needle in untracked content\n")
	matches, err := s.SearchFiles(ctx, id, "NEEDLE")
	if err != nil || !matches.Limited || len(matches.Matches) != 200 {
		t.Fatalf("search length %d, limited %v, error %v", len(matches.Matches), matches.Limited, err)
	}
	if first := matches.Matches[0]; first.Path != filepath.Base(file) || first.Line != 2 || first.Text != "Needle in tracked content" {
		t.Fatalf("unexpected first match: %+v", first)
	}
	empty, err := s.SearchFiles(ctx, id, "absent text")
	if err != nil || len(empty.Matches) != 0 {
		t.Fatalf("expected no results: %+v, %v", empty, err)
	}
	untracked, err := s.SearchFiles(ctx, id, "in untracked content")
	if err != nil || len(untracked.Matches) != 0 {
		t.Fatalf("search included untracked files: %+v, %v", untracked, err)
	}
	write(t, file, "stash changes\n")
	if err := s.EditReference(ctx, id, "stash.save", "preview"); err != nil {
		t.Fatal(err)
	}
	refs, err := s.References(ctx, id)
	if err != nil || len(refs.Stashes) != 1 {
		t.Fatalf("stash list: %+v, %v", refs, err)
	}
	hash, _, _ := strings.Cut(refs.Stashes[0], "\t")
	before := command(t, dir, "status", "--porcelain")
	patch, err := s.StashPatch(ctx, id, hash)
	if err != nil || !strings.Contains(patch, "+stash changes") || !strings.Contains(patch, "+needle in untracked content") {
		t.Fatalf("stash preview lacks tracked or untracked content: %v", err)
	}
	if after := command(t, dir, "status", "--porcelain"); after != before {
		t.Fatalf("preview changed the working tree: before %q, after %q", before, after)
	}
}

func TestUpstreamComparisonPagesAndLoadsOneFile(t *testing.T) {
	s, id, dir := setup(t)
	ctx := context.Background()
	write(t, filepath.Join(dir, "base.txt"), "base\n")
	command(t, dir, "add", ".")
	command(t, dir, "commit", "-m", "base")
	command(t, dir, "branch", "upstream")
	command(t, dir, "branch", "--set-upstream-to=upstream")
	pair, err := s.UpstreamBranches(ctx, id)
	if err != nil || pair != [2]string{"main", "upstream"} {
		t.Fatalf("upstream mapping: %v, %v", pair, err)
	}
	command(t, dir, "remote", "add", "origin", ".")
	command(t, dir, "update-ref", "refs/remotes/origin/shared", "upstream")
	command(t, dir, "branch", "--set-upstream-to=origin/shared")
	pair, err = s.UpstreamBranches(ctx, id)
	if err != nil || pair != [2]string{"main", "refs/remotes/origin/shared"} {
		t.Fatalf("remote upstream mapping: %v, %v", pair, err)
	}
	var stream strings.Builder
	for i := range 401 {
		fmt.Fprintf(&stream, "commit refs/heads/main\ncommitter Test <test@example.invalid> %d +0000\ndata 1\nx\n", 1700000000+i)
		if i == 0 {
			stream.WriteString("from refs/heads/main^0\nM 100644 inline first.txt\ndata 5\nfirst\nM 100644 inline second.txt\ndata 6\nsecond\n")
		}
		stream.WriteByte('\n')
	}
	cmd := exec.Command("git", "fast-import", "--quiet")
	cmd.Dir, cmd.Stdin = dir, strings.NewReader(stream.String())
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fast import: %v: %s", err, output)
	}
	started := time.Now()
	summary, err := s.CompareBranchesSummary(ctx, id, pair[0], pair[1])
	t.Logf("comparison first page and file names: %s", time.Since(started))
	if err != nil || len(summary.Commits) != 200 || !summary.More || len(summary.Paths) != 2 {
		t.Fatalf("comparison summary: commits=%d more=%v paths=%v error=%v", len(summary.Commits), summary.More, summary.Paths, err)
	}
	command(t, dir, "commit", "--allow-empty", "-m", "branch advances during comparison")
	for offset, count := range map[int]int{200: 200, 400: 1} {
		page, err := s.CompareBranchesPage(ctx, id, summary.Left, summary.Right, offset)
		if err != nil || len(page.Commits) != count || page.More != (offset == 200) {
			t.Fatalf("comparison at %d: count=%d more=%v error=%v", offset, len(page.Commits), page.More, err)
		}
	}
	patch, err := s.CompareBranchesFile(ctx, id, summary.Left, summary.Right, "first.txt")
	if err != nil || !strings.Contains(patch, "first.txt") || strings.Contains(patch, "second.txt") {
		t.Fatalf("single-file patch: %q, %v", patch, err)
	}
}
