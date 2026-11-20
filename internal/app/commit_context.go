package app

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"mygittu/internal/commitai"
	git "mygittu/internal/git"
)

func (s *Service) StagedCommitContext(ctx context.Context, id string) (string, error) {
	patch, err := s.Diff(ctx, id, "", true)
	if err != nil && !errors.Is(err, git.ErrOutputLimit) {
		return "", err
	}
	if err == nil && len(patch) <= commitai.MaxPatch && utf8.ValidString(patch) {
		return patch, nil
	}
	stats, statErr := s.Read(ctx, id, "diff", "--cached", "--no-ext-diff", "--no-textconv", "--no-color",
		"--stat=120,96,120", "--dirstat=files,0", "--")
	if statErr != nil {
		return "", statErr
	}
	if err != nil {
		// The captured prefix can end in the middle of a file and omit later files.
		patch = ""
	}
	return compactCommitContext(string(stats.Stdout), patch), nil
}

func compactCommitContext(stats, patch string) string {
	const summaryBudget = 16 << 10
	var result strings.Builder
	result.WriteString("Staged change summary (partial context; omitted diff content is unavailable).\n")
	result.WriteString(commitContextPrefix(stats, summaryBudget))
	result.WriteString("\n")
	if patch == "" {
		return result.String()
	}
	result.WriteString("\nStaged diff excerpts by file:\n")
	files := strings.Split(patch, "\ndiff --git ")
	// Split the remaining space among files, with room for the restored header prefix.
	perFile := (commitai.MaxPatch - result.Len()) / len(files)
	if perFile < 128 {
		return result.String()
	}
	for i, file := range files {
		prefix := ""
		if i > 0 {
			prefix = "diff --git "
		}
		result.WriteString(prefix)
		result.WriteString(commitContextPrefix(file, perFile-len(prefix)-2))
		result.WriteString("\n\n")
	}
	return result.String()
}

func commitContextPrefix(text string, limit int) string {
	text = strings.ToValidUTF8(text, "?")
	if len(text) <= limit {
		return text
	}
	const omitted = "\n[Excerpt truncated]"
	end := limit - len(omitted)
	for !utf8.RuneStart(text[end]) {
		end--
	}
	return text[:end] + omitted
}
