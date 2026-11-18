package app

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

type References struct {
	Stashes []string
	Tags    []string
}

var stashObject = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

func (s *Service) References(ctx context.Context, id string) (References, error) {
	var refs References
	stashes, err := s.Read(ctx, id, "stash", "list", "--format=%H%x09%gd %gs")
	if err != nil {
		return refs, err
	}
	tags, err := s.Read(ctx, id, "tag", "--list")
	if err != nil {
		return refs, err
	}
	lines := func(output []byte) []string {
		value := strings.TrimRight(string(output), "\r\n")
		if value == "" {
			return nil
		}
		return strings.Split(value, "\n")
	}
	refs.Stashes, refs.Tags = lines(stashes.Stdout), lines(tags.Stdout)
	return refs, nil
}

func (s *Service) EditReference(ctx context.Context, id, action, name string) error {
	var args []string
	switch action {
	case "stash.save":
		args = []string{"stash", "push", "--include-untracked", "--message", name}
	case "stash.apply":
		if !stashObject.MatchString(name) {
			return fmt.Errorf("invalid stash selection")
		}
		args = []string{"stash", "apply", "--", name}
	case "tag.create", "tag.delete":
		if name == "" || strings.HasPrefix(name, "-") {
			return fmt.Errorf("invalid tag name")
		}
		if _, err := s.Read(ctx, id, "check-ref-format", "refs/tags/"+name); err != nil {
			return err
		}
		args = []string{"tag", "--", name}
		if action == "tag.delete" {
			args = []string{"tag", "--delete", "--", name}
		}
	default:
		return fmt.Errorf("unsupported reference action")
	}
	_, err := s.Write(ctx, id, args...)
	return err
}
