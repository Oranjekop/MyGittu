package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"

	git "mygittu/internal/git"
	"mygittu/internal/repository"
)

type StageHunk struct {
	Label string
	Token string
	Patch string
}

func (s *Service) worktreePatch(ctx context.Context, id, path string) ([]byte, [][]byte, error) {
	repo, err := s.Registry.Get(id)
	if err != nil {
		return nil, nil, err
	}
	path, err = repository.Relative(repo.Root, path)
	if err != nil {
		return nil, nil, err
	}
	result, err := s.Read(ctx, id, "diff", "--no-ext-diff", "--no-textconv", "--no-color", "--no-renames", "--", path)
	if err != nil {
		return nil, nil, err
	}
	start := bytes.Index(result.Stdout, []byte("\n@@ "))
	if start < 0 {
		return nil, nil, nil
	}
	header := result.Stdout[:start+1]
	if bytes.Contains(header, []byte("\ndeleted file mode ")) || bytes.Contains(header, []byte("\nnew file mode ")) ||
		bytes.Contains(header, []byte("\nold mode ")) || bytes.Count(result.Stdout, []byte("\ndiff --git ")) > 0 {
		return nil, nil, nil
	}
	raw := bytes.Split(result.Stdout[start+1:], []byte("\n@@ "))
	for i := 1; i < len(raw); i++ {
		raw[i] = append([]byte("@@ "), raw[i]...)
	}
	return header, raw, nil
}

func (s *Service) StageHunks(ctx context.Context, id, path string) ([]StageHunk, error) {
	header, hunks, err := s.worktreePatch(ctx, id, path)
	out := make([]StageHunk, 0, len(hunks))
	for _, hunk := range hunks {
		label, _, _ := strings.Cut(string(hunk), "\n")
		out = append(out, StageHunk{Label: label, Token: hunkToken(header, hunk), Patch: strings.ToValidUTF8(string(hunk), "�")})
	}
	return out, err
}

func hunkToken(header, hunk []byte) string {
	hash := sha256.New()
	hash.Write(header)
	hash.Write(hunk)
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func (s *Service) StageSelectedHunk(ctx context.Context, id, path, token string, index int) error {
	return s.applySelectedHunk(ctx, id, path, token, index, false)
}

func (s *Service) RestoreSelectedHunk(ctx context.Context, id, path, token string, index int) error {
	return s.applySelectedHunk(ctx, id, path, token, index, true)
}

func (s *Service) applySelectedHunk(ctx context.Context, id, path, token string, index int, restore bool) error {
	_, err := s.writeLocked(ctx, id, func() (git.Result, error) {
		header, hunks, err := s.worktreePatch(ctx, id, path)
		if err != nil {
			return git.Result{}, err
		}
		if index < 0 || index >= len(hunks) || token != hunkToken(header, hunks[index]) {
			return git.Result{}, fmt.Errorf("selected hunk changed; reload the diff")
		}
		file, err := os.CreateTemp("", "mygittu-stage-*.patch")
		if err != nil {
			return git.Result{}, err
		}
		defer os.Remove(file.Name())
		var patch bytes.Buffer
		patch.Write(header)
		patch.Write(hunks[index])
		if !bytes.HasSuffix(hunks[index], []byte("\n")) {
			patch.WriteByte('\n')
		}
		_, err = file.Write(patch.Bytes())
		closeErr := file.Close()
		if err != nil {
			return git.Result{}, err
		}
		if closeErr != nil {
			return git.Result{}, closeErr
		}
		mode := "--cached"
		if restore {
			mode = "--reverse"
		}
		return s.Read(ctx, id, "apply", mode, "--recount", "--", file.Name())
	})
	return err
}
