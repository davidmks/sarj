// Package status runs the user-configured status hook against worktrees.
//
// The hook is a forge-agnostic shell command. The branch and worktree path
// are exposed as the BRANCH and SARJ_WT_PATH environment variables; the
// command references them via standard shell expansion ($BRANCH,
// $SARJ_WT_PATH). Its trimmed stdout becomes the worktree's status. Non-zero
// exit, empty output, or timeout map to "unknown".
package status

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/davidmks/sarj/internal/exec"
)

// Unknown is the sentinel state returned when a hook fails, times out, or
// produces no output.
const Unknown = "unknown"

// DefaultTimeout bounds each hook invocation when ProbeAll is called without
// an explicit timeout (i.e. zero).
const DefaultTimeout = 10 * time.Second

// MaxParallel caps the number of hook invocations that run concurrently.
// The hook is typically a forge API call (e.g. `gh pr view`); fanning out
// to dozens of those at once trips rate limits.
const MaxParallel = 8

// ErrEmptyOutput is returned in Result.Err when the hook exited cleanly but
// produced no output. Distinguished from a timeout or non-zero exit so
// callers can surface the cause separately if they want to.
var ErrEmptyOutput = errors.New("hook produced no output")

// Item is one worktree to probe.
type Item struct {
	Branch string
	Path   string
}

// Result is the resolved state for one Item, in the order ProbeAll received it.
// State is Unknown when the probe failed; Err carries the cause (timeout,
// non-zero exit, or ErrEmptyOutput) so callers can debug or report.
type Result struct {
	Path  string
	State string
	Err   error
}

// ProbeAll runs the hook for each item, bounded by timeout per call and
// MaxParallel in flight. Results match items by index.
func ProbeAll(ctx context.Context, r exec.Runner, command string, items []Item, timeout time.Duration) []Result {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	out := make([]Result, len(items))
	sem := make(chan struct{}, MaxParallel)
	var wg sync.WaitGroup

	for i, it := range items {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i] = probeOne(ctx, r, command, it, timeout)
		})
	}
	wg.Wait()
	return out
}

func probeOne(ctx context.Context, r exec.Runner, command string, it Item, timeout time.Duration) Result {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	env := []string{"BRANCH=" + it.Branch, "SARJ_WT_PATH=" + it.Path}
	out, err := r.RunWithEnv(ctx, env, "sh", "-c", command)
	if err != nil {
		return Result{Path: it.Path, State: Unknown, Err: fmt.Errorf("hook failed: %w", err)}
	}
	state := strings.TrimSpace(out)
	if state == "" {
		return Result{Path: it.Path, State: Unknown, Err: ErrEmptyOutput}
	}
	return Result{Path: it.Path, State: state}
}
