// Package exec provides an abstraction over os/exec for testability.
package exec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"strings"
	"time"
)

// killPipeDelay bounds how long Wait will linger on stdout/stderr pipes
// after Cancel. A compound `sh -c "A; B"` may have orphaned grandchildren
// that hold the pipes open indefinitely; this caps that window so timeouts
// observed by callers are close to what they configured.
const killPipeDelay = 100 * time.Millisecond

// Runner abstracts command execution so callers can be tested
// without actually running git, tmux, etc.
type Runner interface {
	// Run executes a command bounded by ctx and returns its trimmed
	// combined stdout/stderr output. When ctx is canceled or its deadline
	// passes, the underlying process is killed.
	Run(ctx context.Context, name string, args ...string) (string, error)

	// RunWithEnv is like Run but appends extra env vars to the parent
	// environment and returns trimmed *stdout only* — stderr is surfaced
	// via the returned error. Used by the status hook so diagnostics
	// (e.g. `gh` warnings) don't leak into the state token.
	RunWithEnv(ctx context.Context, env []string, name string, args ...string) (string, error)

	// RunInteractive connects the command's stdin/stdout/stderr to the
	// terminal — used for things like tmux attach that need a live TTY.
	RunInteractive(ctx context.Context, name string, args ...string) error
}

// DefaultRunner implements Runner using os/exec.
type DefaultRunner struct {
	// Dir sets the working directory for commands. Empty means current dir.
	Dir string
}

// Run executes a command bounded by ctx and returns its trimmed combined output.
func (r *DefaultRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := osexec.CommandContext(ctx, name, args...)
	if r.Dir != "" {
		cmd.Dir = r.Dir
	}

	out, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(out))

	if err != nil {
		return result, fmt.Errorf("running %s %s: %w: %s", name, strings.Join(args, " "), err, result)
	}

	return result, nil
}

// RunWithEnv executes a command with extra env vars and returns its trimmed
// stdout. Stderr is captured separately and folded into the error on failure.
// WaitDelay ensures Wait returns near the context deadline even when a
// compound shell command leaves grandchildren holding the output pipes open.
func (r *DefaultRunner) RunWithEnv(ctx context.Context, env []string, name string, args ...string) (string, error) {
	cmd := osexec.CommandContext(ctx, name, args...)
	if r.Dir != "" {
		cmd.Dir = r.Dir
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.WaitDelay = killPipeDelay

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	result := strings.TrimSpace(string(out))

	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return result, fmt.Errorf("running %s %s: %w: %s", name, strings.Join(args, " "), err, msg)
		}
		return result, fmt.Errorf("running %s %s: %w", name, strings.Join(args, " "), err)
	}

	return result, nil
}

// RunInteractive runs a command connected to the terminal's stdin/stdout/stderr.
func (r *DefaultRunner) RunInteractive(ctx context.Context, name string, args ...string) error {
	cmd := osexec.CommandContext(ctx, name, args...)
	if r.Dir != "" {
		cmd.Dir = r.Dir
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running interactive %s %s: %w", name, strings.Join(args, " "), err)
	}

	return nil
}
