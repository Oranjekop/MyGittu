package app

import (
	"context"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
)

type Identity struct {
	Name           string `json:"name"`
	Email          string `json:"email"`
	Scope          string `json:"scope"`
	EffectiveName  string `json:"effective_name"`
	EffectiveEmail string `json:"effective_email"`
}

func identityScope(scope string) (string, error) {
	switch scope {
	case "local":
		return "--local", nil
	case "global":
		return "--global", nil
	}
	return "", fmt.Errorf("invalid identity scope")
}
func (s *Service) Identity(ctx context.Context, id, scope string) (Identity, error) {
	flag, e := identityScope(scope)
	if e != nil {
		return Identity{}, e
	}
	repo := ""
	if id != "" {
		info, e := s.Registry.Get(id)
		if e != nil {
			return Identity{}, e
		}
		repo = info.Root
	} else if scope == "local" {
		return Identity{}, fmt.Errorf("open a repository first")
	}
	read := func(args ...string) (string, error) {
		r, e := s.Runner.Run(ctx, repo, args...)
		if e != nil && r.ExitCode != 1 {
			return "", e
		}
		return strings.TrimSuffix(strings.TrimSuffix(string(r.Stdout), "\n"), "\r"), nil
	}
	v := Identity{Scope: scope}
	v.Name, e = read("config", flag, "--get", "user.name")
	if e != nil {
		return v, e
	}
	v.Email, e = read("config", flag, "--get", "user.email")
	if e != nil {
		return v, e
	}
	if scope == "global" {
		v.EffectiveName = v.Name
		v.EffectiveEmail = v.Email
		return v, nil
	}
	v.EffectiveName, e = read("config", "--get", "user.name")
	if e != nil {
		return v, e
	}
	v.EffectiveEmail, e = read("config", "--get", "user.email")
	return v, e
}
func (s *Service) UpdateIdentity(ctx context.Context, id string, v Identity) (Identity, error) {
	_, e := identityScope(v.Scope)
	if e != nil {
		return v, e
	}
	v.Name = strings.TrimSpace(v.Name)
	v.Email = strings.TrimSpace(v.Email)
	if v.Name == "" || len(v.Name) > 256 || strings.ContainsAny(v.Name, "\x00\r\n<>") {
		return v, fmt.Errorf("enter a valid author name")
	}
	address, e := mail.ParseAddress(v.Email)
	if e != nil || address.Address != v.Email || len(v.Email) > 254 || strings.ContainsAny(v.Email, "\x00\r\n<>") {
		return v, fmt.Errorf("enter a valid email address")
	}
	key := id
	if v.Scope == "global" {
		key = "identity-global"
	}
	lockValue, _ := s.locks.LoadOrStore(key, make(chan struct{}, 1))
	lock := lockValue.(chan struct{})
	select {
	case lock <- struct{}{}:
		defer func() { <-lock }()
	case <-ctx.Done():
		return v, ctx.Err()
	}
	repo := ""
	if id != "" {
		info, e := s.Registry.Get(id)
		if e != nil {
			return v, e
		}
		repo = info.Root
	} else if v.Scope == "local" {
		return v, fmt.Errorf("open a repository first")
	}
	var target string
	if v.Scope == "local" {
		result, e := s.Runner.Run(ctx, repo, "rev-parse", "--path-format=absolute", "--git-common-dir")
		if e != nil {
			return v, e
		}
		target = filepath.Join(strings.TrimSpace(string(result.Stdout)), "config")
	} else {
		result, e := s.Runner.Run(ctx, repo, "var", "GIT_CONFIG_GLOBAL")
		if e != nil {
			return v, e
		}
		paths := strings.Split(strings.TrimSuffix(string(result.Stdout), "\n"), "\n")
		if len(paths) < 1 || len(paths) > 2 {
			return v, fmt.Errorf("unsupported global config path")
		}
		target = paths[len(paths)-1]
		if len(paths) == 2 {
			if _, e := os.Stat(target); os.IsNotExist(e) {
				if _, e := os.Stat(paths[0]); e == nil {
					target = paths[0]
				}
			}
		}
	}
	target = filepath.FromSlash(strings.TrimSuffix(target, "\r"))
	if resolved, e := filepath.EvalSymlinks(target); e == nil {
		target = resolved
	}
	if !filepath.IsAbs(target) {
		return v, fmt.Errorf("absolute Git config path required")
	}
	if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
		return v, e
	}
	lockPath := target + ".lock"
	locked, e := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return v, fmt.Errorf("lock Git configuration: %w", e)
	}
	defer os.Remove(lockPath)
	data, readErr := os.ReadFile(target)
	if readErr != nil && !os.IsNotExist(readErr) {
		locked.Close()
		return v, readErr
	}
	if info, err := os.Stat(target); err == nil {
		_ = locked.Chmod(info.Mode().Perm())
	}
	if _, e = locked.Write(data); e != nil {
		locked.Close()
		return v, e
	}
	if e = locked.Close(); e != nil {
		return v, e
	}
	// Git edits the private candidate using its own parser and lock protocol.
	// The original stays untouched until both fields have been validated/written.
	for _, pair := range [][2]string{{"user.name", v.Name}, {"user.email", v.Email}} {
		if _, e = s.Runner.Run(ctx, repo, "config", "--file", lockPath, "--replace-all", "--", pair[0], pair[1]); e != nil {
			return v, e
		}
	}
	if e = ctx.Err(); e != nil {
		return v, e
	}
	if e = os.Rename(lockPath, target); e != nil {
		return v, e
	}
	return s.Identity(ctx, id, v.Scope)
}
