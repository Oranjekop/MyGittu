package app

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"mygittu/internal/repository"
)

func (s *Service) Files(ctx context.Context, id string) ([]string, error) {
	result, err := s.Read(ctx, id, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--deduplicate")
	if err != nil {
		return nil, err
	}
	files := make([]string, 0)
	for path := range strings.SplitSeq(string(result.Stdout), "\x00") {
		if path != "" {
			files = append(files, path)
		}
	}
	sort.Strings(files)
	return files, nil
}

func (s *Service) PreviewFile(id, path string) (Comparison, error) {
	repo, err := s.Registry.Get(id)
	if err != nil {
		return Comparison{}, err
	}
	path, err = repository.Relative(repo.Root, path)
	if err != nil {
		return Comparison{}, err
	}
	for _, part := range strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' }) {
		if strings.EqualFold(part, ".git") {
			return Comparison{}, fmt.Errorf("Git metadata cannot be previewed")
		}
	}
	data, binary, err := readWorktreeFile(repo.Root, path)
	if err != nil {
		return Comparison{}, err
	}
	if len(data) > 256<<10 || bytes.Count(data, []byte{'\n'}) > 8000 {
		return Comparison{}, fmt.Errorf("file preview is limited to 256 KiB and 8000 lines")
	}
	if binary || bytes.ContainsRune(data, 0) || !utf8.Valid(data) {
		return Comparison{Binary: true}, nil
	}
	return Comparison{New: strings.ReplaceAll(string(data), "\r\n", "\n")}, nil
}
