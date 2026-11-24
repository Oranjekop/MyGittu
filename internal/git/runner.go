package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type Result struct {
	Stdout, Stderr []byte
	ExitCode       int
}
type Runner struct {
	Binary  string
	Timeout time.Duration
}

var ErrOutputLimit = errors.New("Git output exceeds capture limit")

func NewRunner(binary string) (*Runner, error) {
	if binary == "" {
		binary = "git"
	}
	p, e := exec.LookPath(binary)
	if e != nil {
		return nil, e
	}
	r := &Runner{p, 2 * time.Minute}
	_, e = r.Run(context.Background(), "", "--version")
	return r, e
}

type limitedBuffer struct {
	buffer   bytes.Buffer
	max      int
	exceeded bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if b.buffer.Len()+n > b.max {
		b.exceeded = true
		return 0, fmt.Errorf("Git output exceeds %d bytes", b.max)
	}
	return b.buffer.Write(p)
}
func (r *Runner) Run(ctx context.Context, repo string, args ...string) (Result, error) {
	return r.run(ctx, repo, nil, args...)
}

func (r *Runner) run(ctx context.Context, repo string, env []string, args ...string) (Result, error) {
	return r.runInput(ctx, repo, env, "", args...)
}

func (r *Runner) runInput(ctx context.Context, repo string, env []string, input string, args ...string) (Result, error) {
	return r.runOutput(ctx, repo, env, input, nil, args...)
}

func (r *Runner) RunTo(ctx context.Context, repo string, output io.Writer, args ...string) error {
	_, err := r.runOutput(ctx, repo, nil, "", output, args...)
	return err
}

func (r *Runner) runOutput(ctx context.Context, repo string, env []string, input string, output io.Writer, args ...string) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	// Disable interactive prompts; use installed credential helpers for remote operations.
	cmd := exec.CommandContext(ctx, r.Binary, append([]string{"-c", "core.fsmonitor=false"}, args...)...)
	configureProcess(cmd)
	cmd.WaitDelay = 2 * time.Second
	cmd.Dir = repo
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_LITERAL_PATHSPECS=1", "GIT_PAGER=cat")
	cmd.Env = append(cmd.Env, env...)
	out, errout := &limitedBuffer{max: 8 << 20}, &limitedBuffer{max: 64 << 10}
	cmd.Stdout = out
	if output != nil {
		cmd.Stdout = output
	}
	cmd.Stderr = errout
	err := cmd.Run()
	res := Result{out.buffer.Bytes(), errout.buffer.Bytes(), 0}
	if err != nil {
		res.ExitCode = -1
		if cmd.ProcessState != nil {
			res.ExitCode = cmd.ProcessState.ExitCode()
		}
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		if out.exceeded {
			return res, fmt.Errorf("%w (%d bytes)", ErrOutputLimit, out.max)
		}
		message := Redact(strings.TrimSpace(string(res.Stderr)))
		if len(message) > 2048 {
			message = message[:2048]
		}
		if message == "" {
			message = err.Error()
		}
		return res, fmt.Errorf("Git command failed (exit %d): %s", res.ExitCode, message)
	}
	return res, nil
}

var credentials = regexp.MustCompile(`(?i)(https?://)[^\s/@]+(?::[^\s/@]*)?@`)
var tokens = regexp.MustCompile(`(?i)(authorization[:= ]+|password[:= ]+|token[:= ]+|access_token=)[^\s&]+`)

func Redact(s string) string {
	return tokens.ReplaceAllString(credentials.ReplaceAllString(s, "${1}[redacted]@"), "${1}[redacted]")
}
