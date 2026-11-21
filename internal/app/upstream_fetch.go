package app

import (
	"context"
	"strings"

	git "mygittu/internal/git"
)

// FetchUpstream refreshes only the configured tracking ref of the checked-out branch.
func (s *Service) FetchUpstream(ctx context.Context, id string) (bool, error) {
	fetched := false
	_, err := s.writeLocked(ctx, id, func() (git.Result, error) {
		head, err := s.Read(ctx, id, "symbolic-ref", "--quiet", "HEAD")
		if err != nil {
			return git.Result{}, nil // Detached HEAD has no upstream.
		}
		ref := strings.TrimSpace(string(head.Stdout))
		info, err := s.Read(ctx, id, "for-each-ref",
			"--format=%(upstream:remotename)%00%(upstream:remoteref)%00%(upstream)", ref)
		if err != nil {
			return info, err
		}
		fields := strings.Split(strings.TrimSpace(string(info.Stdout)), "\x00")
		if len(fields) != 3 || fields[0] == "" || fields[0] == "." ||
			!strings.HasPrefix(fields[1], "refs/heads/") || !strings.HasPrefix(fields[2], "refs/remotes/") {
			return git.Result{}, nil
		}
		result, err := s.Read(ctx, id, "-c", "credential.interactive=false",
			"fetch", "--no-tags", "--no-prune", "--no-recurse-submodules", "--no-auto-maintenance",
			"--no-write-fetch-head", "--refmap=", "--", fields[0], "+"+fields[1]+":"+fields[2])
		fetched = err == nil
		return result, err
	})
	return fetched, err
}
