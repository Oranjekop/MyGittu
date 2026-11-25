package app

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"

	git "mygittu/internal/git"
)

type ReflogEntry struct {
	Hash, Selector, Summary string
}

func (s *Service) Reflog(ctx context.Context, id string) ([]ReflogEntry, error) {
	if result, err := s.Read(ctx, id, "reflog", "exists", "HEAD"); err != nil {
		if result.ExitCode == 1 {
			return nil, nil
		}
		return nil, err
	}
	result, err := s.Read(ctx, id, "reflog", "-n", "200", "--format=%H%x09%gD%x09%gs", "HEAD")
	if err != nil {
		return nil, err
	}
	var entries []ReflogEntry
	for line := range strings.SplitSeq(string(result.Stdout), "\n") {
		fields := strings.SplitN(strings.TrimSuffix(line, "\r"), "\t", 3)
		if len(fields) == 3 && commitHash.MatchString(fields[0]) {
			entries = append(entries, ReflogEntry{fields[0], fields[1], fields[2]})
		}
	}
	return entries, nil
}

func (s *Service) BranchAt(ctx context.Context, id, name, hash string) error {
	if name == "" || strings.HasPrefix(name, "-") || !commitHash.MatchString(hash) {
		return fmt.Errorf("invalid branch name or commit")
	}
	_, err := s.Write(ctx, id, "branch", "--", name, hash)
	return err
}

const MaxPatchBytes = 1 << 20

type PatchCheck struct{ Summary, Token string }

func (s *Service) ExportPatch(ctx context.Context, id string, staged bool) (string, error) {
	args := []string{"diff", "--binary", "--no-ext-diff", "--no-textconv", "--no-color"}
	if staged {
		args = append(args, "--cached")
	}
	result, err := s.Read(ctx, id, append(args, "--")...)
	if err == nil && len(result.Stdout) > MaxPatchBytes {
		err = fmt.Errorf("patch exceeds %d bytes", MaxPatchBytes)
	}
	return string(result.Stdout), err
}

func (s *Service) ApplyPatch(ctx context.Context, id, patch, token string) (PatchCheck, error) {
	if patch == "" || len(patch) > MaxPatchBytes {
		return PatchCheck{}, fmt.Errorf("patch must contain between 1 and %d bytes", MaxPatchBytes)
	}
	repo, err := s.Registry.Get(id)
	if err != nil {
		return PatchCheck{}, err
	}
	check := PatchCheck{Token: fmt.Sprintf("%x", sha256.Sum256([]byte(patch)))}
	run := func(flags ...string) (git.Result, error) {
		return s.Runner.RunWithInput(ctx, repo.Root, patch, append(append([]string{"apply"}, flags...), "-")...)
	}
	_, err = s.writeLocked(ctx, id, func() (git.Result, error) {
		if result, err := run("--check"); err != nil {
			return result, err
		}
		if token != "" {
			if token != check.Token {
				return git.Result{}, fmt.Errorf("patch changed after preview")
			}
			return run()
		}
		result, err := run("--stat", "--summary")
		check.Summary = string(result.Stdout)
		return result, err
	})
	return check, err
}

type AdvancedState struct {
	Rebasing, Bisecting bool
}

func (s *Service) AdvancedState(ctx context.Context, id string) AdvancedState {
	return AdvancedState{
		Rebasing:  s.GitStateExists(ctx, id, "rebase-merge") || s.GitStateExists(ctx, id, "rebase-apply"),
		Bisecting: s.GitStateExists(ctx, id, "BISECT_START"),
	}
}

type RebaseStep struct{ Action, Hash string }

func (s *Service) rebaseCommits(ctx context.Context, id, base string) ([]RebaseStep, error) {
	if !commitHash.MatchString(base) {
		return nil, fmt.Errorf("select a full base commit hash")
	}
	if _, err := s.Read(ctx, id, "merge-base", "--is-ancestor", base, "HEAD"); err != nil {
		return nil, err
	}
	result, err := s.Read(ctx, id, "rev-list", "--reverse", base+"..HEAD")
	if err != nil {
		return nil, err
	}
	merges, err := s.Read(ctx, id, "rev-list", "--merges", "-n", "1", base+"..HEAD")
	if err != nil {
		return nil, err
	}
	hashes := strings.Fields(string(result.Stdout))
	if len(merges.Stdout) > 0 || len(hashes) == 0 || len(hashes) > 200 {
		return nil, fmt.Errorf("interactive rebase requires 1–200 linear commits")
	}
	steps := make([]RebaseStep, 0, len(hashes))
	for _, hash := range hashes {
		steps = append(steps, RebaseStep{"pick", hash})
	}
	return steps, nil
}

func (s *Service) RebasePlan(ctx context.Context, id, base string) ([]RebaseStep, error) {
	return s.rebaseCommits(ctx, id, base)
}

func (s *Service) Rebase(ctx context.Context, id, action, base string, steps []RebaseStep) error {
	repo, err := s.Registry.Get(id)
	if err != nil {
		return err
	}
	_, err = s.writeLocked(ctx, id, func() (git.Result, error) {
		state := s.AdvancedState(ctx, id)
		switch action {
		case "continue", "skip", "abort":
			if !state.Rebasing {
				return git.Result{}, fmt.Errorf("no rebase in progress")
			}
			return s.Runner.RunWithEnv(ctx, repo.Root, []string{"GIT_EDITOR=true"}, "rebase", "--"+action)
		case "start":
			if state.Rebasing || state.Bisecting {
				return git.Result{}, fmt.Errorf("another Git operation is in progress")
			}
		default:
			return git.Result{}, fmt.Errorf("unsupported rebase action")
		}
		expected, err := s.rebaseCommits(ctx, id, base)
		if err != nil {
			return git.Result{}, err
		}
		if len(steps) != len(expected) {
			return git.Result{}, fmt.Errorf("rebase plan is stale or incomplete")
		}
		remaining := make(map[string]bool, len(expected))
		for _, step := range expected {
			remaining[step.Hash] = true
		}
		var todo strings.Builder
		hasPrevious := false
		for _, step := range steps {
			if !remaining[step.Hash] || (step.Action != "pick" && step.Action != "drop" && step.Action != "fixup") {
				return git.Result{}, fmt.Errorf("invalid rebase plan")
			}
			if step.Action == "fixup" && !hasPrevious {
				return git.Result{}, fmt.Errorf("fixup requires a preceding kept commit")
			}
			hasPrevious = hasPrevious || step.Action == "pick"
			delete(remaining, step.Hash)
			fmt.Fprintf(&todo, "%s %s\n", step.Action, step.Hash)
		}
		tmp, err := os.CreateTemp("", "mygittu-rebase-*")
		if err != nil {
			return git.Result{}, err
		}
		defer os.Remove(tmp.Name())
		_, err = tmp.WriteString(todo.String())
		closeErr := tmp.Close()
		if err != nil {
			return git.Result{}, err
		}
		if closeErr != nil {
			return git.Result{}, closeErr
		}
		return s.Runner.RunWithEnv(ctx, repo.Root, []string{
			"GIT_EDITOR=true", `GIT_SEQUENCE_EDITOR=cat "$MYGITTU_REBASE_TODO" >`, "MYGITTU_REBASE_TODO=" + tmp.Name(),
		}, "rebase", "--interactive", "--no-autostash", "--no-autosquash", "--no-update-refs", base)
	})
	return err
}

func (s *Service) Bisect(ctx context.Context, id, action, bad, good string) (string, error) {
	result, err := s.writeLocked(ctx, id, func() (git.Result, error) {
		state := s.AdvancedState(ctx, id)
		switch action {
		case "start":
			if state.Rebasing || state.Bisecting || !commitHash.MatchString(bad) || !commitHash.MatchString(good) || bad == good {
				return git.Result{}, fmt.Errorf("invalid bisect range or operation in progress")
			}
			if _, err := s.Read(ctx, id, "merge-base", "--is-ancestor", good, bad); err != nil {
				return git.Result{}, err
			}
			return s.Read(ctx, id, "bisect", "start", bad, good, "--")
		case "good", "bad", "skip", "reset":
			if !state.Bisecting {
				return git.Result{}, fmt.Errorf("no bisect in progress")
			}
			return s.Read(ctx, id, "bisect", action)
		default:
			return git.Result{}, fmt.Errorf("unsupported bisect action")
		}
	})
	return string(result.Stdout), err
}
