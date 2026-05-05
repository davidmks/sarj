// Package status runs the user-configured status hook against worktrees.
//
// The hook is a forge-agnostic shell command templated with {{.Branch}} and
// {{.Path}}. Its trimmed stdout becomes the worktree's status. Non-zero exit,
// empty output, or timeout map to "unknown".
package status

import (
	"context"
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

// Item is one worktree to probe.
type Item struct {
	Branch string
	Path   string
}

// Result is the resolved state for one Item, in the order ProbeAll received it.
type Result struct {
	Path  string
	State string
}

// Build substitutes {{.Branch}} and {{.Path}} in template.
func Build(template, branch, path string) string {
	s := strings.ReplaceAll(template, "{{.Branch}}", branch)
	return strings.ReplaceAll(s, "{{.Path}}", path)
}

// ProbeAll runs the templated command for each item in parallel, each call
// bounded by timeout (DefaultTimeout when zero). Results match items by index.
func ProbeAll(ctx context.Context, r exec.Runner, template string, items []Item, timeout time.Duration) []Result {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	out := make([]Result, len(items))
	var wg sync.WaitGroup
	for i, it := range items {
		wg.Add(1)
		go func(i int, it Item) {
			defer wg.Done()
			out[i] = Result{Path: it.Path, State: probeOne(ctx, r, Build(template, it.Branch, it.Path), timeout)}
		}(i, it)
	}
	wg.Wait()
	return out
}

func probeOne(parent context.Context, r exec.Runner, command string, timeout time.Duration) string {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	out, err := r.RunContext(ctx, "sh", "-c", command)
	if err != nil {
		return Unknown
	}
	state := strings.TrimSpace(out)
	if state == "" {
		return Unknown
	}
	return state
}
