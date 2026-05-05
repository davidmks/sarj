// Package exec provides an abstraction over os/exec for testability.
package exec

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"strings"
)

// Runner abstracts command execution so callers can be tested
// without actually running git, tmux, etc.
type Runner interface {
	// Run executes a command and returns its combined stdout/stderr output.
	Run(name string, args ...string) (string, error)

	// RunContext is like Run but bounded by ctx. When ctx is canceled or its
	// deadline passes, the underlying process is killed and the call returns
	// whatever output was produced along with the context error.
	RunContext(ctx context.Context, name string, args ...string) (string, error)

	// RunInteractive connects the command's stdin/stdout/stderr to the
	// terminal — used for things like tmux attach that need a live TTY.
	RunInteractive(name string, args ...string) error
}

// DefaultRunner implements Runner using os/exec.
type DefaultRunner struct {
	// Dir sets the working directory for commands. Empty means current dir.
	Dir string
}

// Run executes a command and returns its trimmed output.
func (r *DefaultRunner) Run(name string, args ...string) (string, error) {
	return r.RunContext(context.Background(), name, args...)
}

// RunContext executes a command bounded by ctx and returns its trimmed output.
func (r *DefaultRunner) RunContext(ctx context.Context, name string, args ...string) (string, error) {
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

// RunInteractive runs a command connected to the terminal's stdin/stdout/stderr.
func (r *DefaultRunner) RunInteractive(name string, args ...string) error {
	cmd := osexec.Command(name, args...)
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
