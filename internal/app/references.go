package app

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type References struct {
	Stashes  []string
	Tags     []string
	TagTimes map[string]TagTime
}

type TagTime struct {
	Unix      int64
	Annotated bool
}

var stashObject = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

func (s *Service) StashPatch(ctx context.Context, id, hash string) (string, error) {
	if !stashObject.MatchString(hash) {
		return "", fmt.Errorf("invalid stash selection")
	}
	result, err := s.Read(ctx, id, "stash", "show", "--include-untracked", "--patch", "--no-ext-diff", "--no-textconv", "--no-color", hash, "--")
	return string(result.Stdout), err
}

func (s *Service) References(ctx context.Context, id string) (References, error) {
	var refs References
	stashes, err := s.Read(ctx, id, "stash", "list", "--format=%H%x09%gd %gs")
	if err != nil {
		return refs, err
	}
	tags, err := s.Read(ctx, id, "for-each-ref", "--sort=-version:refname",
		"--format=%(refname:lstrip=2)%09%(objecttype)%09%(creatordate:unix)", "refs/tags")
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
	refs.Stashes = lines(stashes.Stdout)
	refs.TagTimes = make(map[string]TagTime)
	for _, line := range lines(tags.Stdout) {
		name, rest, _ := strings.Cut(line, "\t")
		kind, stamp, _ := strings.Cut(rest, "\t")
		refs.Tags = append(refs.Tags, name)
		if created, err := strconv.ParseInt(strings.TrimSpace(stamp), 10, 64); err == nil {
			refs.TagTimes[name] = TagTime{Unix: created, Annotated: kind == "tag"}
		}
	}
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
