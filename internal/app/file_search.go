package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	git "mygittu/internal/git"
)

type FileMatch struct {
	Path, Text string
	Line       int
}

type FileSearch struct {
	Matches []FileMatch
	Limited bool
}

func (s *Service) SearchFiles(ctx context.Context, id, query string) (FileSearch, error) {
	const limit = 200
	var matches FileSearch
	if query == "" || len(query) > 1024 || strings.ContainsAny(query, "\x00\r\n") {
		return matches, fmt.Errorf("invalid file search query")
	}
	result, err := s.Read(ctx, id, "grep", "-I", "-n", "-z", "--no-color", "--no-heading", "--no-break", "--max-count=201", "-F", "-i", "-e", query, "--")
	if result.ExitCode == 1 && ctx.Err() == nil && len(result.Stderr) == 0 {
		return matches, nil
	}
	if err != nil && !errors.Is(err, git.ErrOutputLimit) {
		return matches, err
	}
	matches.Limited = err != nil
	for data := string(result.Stdout); data != ""; {
		path, tail, hasPath := strings.Cut(data, "\x00")
		line, tail, hasLine := strings.Cut(tail, "\x00")
		snippet, next, complete := strings.Cut(tail, "\n")
		if !hasPath || !hasLine || !complete {
			break
		}
		if len(matches.Matches) == limit {
			matches.Limited = true
			break
		}
		number, err := strconv.Atoi(line)
		if err != nil || number < 1 {
			return matches, fmt.Errorf("invalid Git search result")
		}
		snippet = strings.TrimSpace(snippet)
		snippet = strings.ToValidUTF8(snippet[:min(len(snippet), 512)], "�")
		matches.Matches = append(matches.Matches, FileMatch{Path: path, Text: snippet, Line: number})
		data = next
	}
	return matches, nil
}
