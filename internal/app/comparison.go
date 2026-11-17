package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mygittu/internal/repository"
	"os"
	"strings"
	"unicode/utf8"
)

type Comparison struct {
	Old    string `json:"old"`
	New    string `json:"new"`
	Binary bool   `json:"binary"`
}

func (s *Service) Comparison(ctx context.Context, id, path, original string, staged bool) (Comparison, error) {
	repo, err := s.Registry.Get(id)
	if err != nil {
		return Comparison{}, err
	}
	path, err = repository.Relative(repo.Root, path)
	if err != nil {
		return Comparison{}, err
	}
	if original == "" {
		original = path
	}
	original, err = repository.Relative(repo.Root, original)
	if err != nil {
		return Comparison{}, err
	}
	var oldText, newText []byte
	if staged {
		head, headErr := s.Read(ctx, id, "rev-parse", "--verify", "--quiet", "HEAD")
		if headErr == nil {
			listing, err := s.Read(ctx, id, "ls-tree", "-z", "HEAD", "--", original)
			if err != nil {
				return Comparison{}, err
			}
			fields := bytes.Fields(bytes.SplitN(listing.Stdout, []byte{'\t'}, 2)[0])
			if len(fields) > 0 {
				if len(fields) != 3 || string(fields[1]) != "blob" {
					return Comparison{Binary: true}, nil
				}
				blob, err := s.Read(ctx, id, "cat-file", "blob", string(fields[2]))
				if err != nil {
					return Comparison{}, err
				}
				oldText = blob.Stdout
			}
		} else if head.ExitCode != 1 {
			return Comparison{}, headErr
		}
	}
	index, err := s.Read(ctx, id, "ls-files", "--stage", "-z", "--", path)
	if err != nil {
		return Comparison{}, err
	}
	var indexBlob []byte
	for _, entry := range bytes.Split(index.Stdout, []byte{0}) {
		fields := bytes.Fields(bytes.SplitN(entry, []byte{'\t'}, 2)[0])
		if len(fields) != 3 {
			continue
		}
		if string(fields[0]) == "160000" {
			return Comparison{Binary: true}, nil
		}
		if string(fields[2]) == "0" || string(fields[2]) == "2" {
			blob, err := s.Read(ctx, id, "cat-file", "blob", string(fields[1]))
			if err != nil {
				return Comparison{}, err
			}
			indexBlob = blob.Stdout
			if string(fields[2]) == "0" {
				break
			}
		}
	}
	if staged {
		newText = indexBlob
	} else {
		oldText = indexBlob
		root, err := os.OpenRoot(repo.Root)
		if err != nil {
			return Comparison{}, err
		}
		defer root.Close()
		info, err := root.Lstat(path)
		if os.IsNotExist(err) {
			newText = nil
		} else if err != nil {
			return Comparison{}, err
		} else if info.Mode()&os.ModeSymlink != 0 {
			target, err := root.Readlink(path)
			if err != nil {
				return Comparison{}, err
			}
			newText = []byte(target)
		} else if info.Mode().IsRegular() {
			file, err := root.Open(path)
			if err != nil {
				return Comparison{}, err
			}
			newText, err = io.ReadAll(io.LimitReader(file, (256<<10)+1))
			file.Close()
			if err != nil {
				return Comparison{}, err
			}
		} else {
			return Comparison{Binary: true}, nil
		}
	}
	if len(oldText) > 256<<10 || len(newText) > 256<<10 || bytes.Count(oldText, []byte{'\n'}) > 8000 || bytes.Count(newText, []byte{'\n'}) > 8000 {
		return Comparison{}, fmt.Errorf("file preview is limited to 256 KiB and 8000 lines per side")
	}
	if bytes.ContainsRune(oldText, 0) || bytes.ContainsRune(newText, 0) || !utf8.Valid(oldText) || !utf8.Valid(newText) {
		return Comparison{Binary: true}, nil
	}
	return Comparison{Old: strings.ReplaceAll(string(oldText), "\r\n", "\n"), New: strings.ReplaceAll(string(newText), "\r\n", "\n")}, nil
}
