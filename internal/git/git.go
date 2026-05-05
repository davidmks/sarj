// Package git wraps git CLI operations needed for worktree management.
package git

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CommandRunner is the subset of exec.Runner that this package needs.
type CommandRunner interface {
	Run(name string, args ...string) (string, error)
}

// RepoRoot returns the absolute path to the repository root.
func RepoRoot(r CommandRunner) (string, error) {
	out, err := r.Run("git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("finding repo root: %w", err)
	}
	return out, nil
}

// MainWorktree returns the path of the main (first) worktree.
// Git always lists the main worktree first in `git worktree list`.
func MainWorktree(r CommandRunner) (string, error) {
	out, err := r.Run("git", "worktree", "list", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("listing worktrees: %w", err)
	}

	// First line of porcelain output is "worktree /path/to/main"
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "worktree ") {
			return strings.TrimPrefix(line, "worktree "), nil
		}
	}

	return "", fmt.Errorf("no worktree found in output")
}

// DefaultBranch detects the default branch by checking the remote HEAD ref.
// Falls back to "main" if detection fails.
func DefaultBranch(r CommandRunner) string {
	out, err := r.Run("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	if err != nil {
		return "main"
	}

	// Output is like "refs/remotes/origin/main"
	parts := strings.Split(strings.TrimSpace(out), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return "main"
}

// Fetch runs git fetch for the given remote.
func Fetch(r CommandRunner, remote string) error {
	_, err := r.Run("git", "fetch", remote)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", remote, err)
	}
	return nil
}

// BranchExists checks if a local branch with the given name exists.
func BranchExists(r CommandRunner, name string) bool {
	_, err := r.Run("git", "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	return err == nil
}

// RemoteRefExists checks if a remote-tracking ref exists (e.g., "origin/main").
func RemoteRefExists(r CommandRunner, ref string) bool {
	_, err := r.Run("git", "show-ref", "--verify", "--quiet", "refs/remotes/"+ref)
	return err == nil
}

// CommitsBehind returns the number of commits local is behind remote.
// Both refs are used as-is (e.g., "main", "origin/main"). Returns 0 if the comparison fails.
func CommitsBehind(r CommandRunner, local, remote string) int {
	out, err := r.Run("git", "rev-list", "--count", local+".."+remote)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0
	}
	return n
}

// Dirty reports whether the worktree at path has uncommitted changes
// (modified, staged, or untracked).
func Dirty(r CommandRunner, path string) (bool, error) {
	out, err := r.Run("git", "-C", path, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("checking dirty state: %w", err)
	}
	return strings.TrimSpace(out) != "", nil
}

// HeadInfo returns the subject and committer date of HEAD for the worktree at
// path. Date is parsed from strict ISO-8601 (%cI), which is RFC3339-compatible.
func HeadInfo(r CommandRunner, path string) (subject string, date time.Time, err error) {
	out, err := r.Run("git", "-C", path, "log", "-1", "--format=%cI%n%s")
	if err != nil {
		return "", time.Time{}, fmt.Errorf("reading head info: %w", err)
	}
	lines := strings.SplitN(strings.TrimRight(out, "\n"), "\n", 2)
	if len(lines) < 2 {
		return "", time.Time{}, fmt.Errorf("unexpected log output: %q", out)
	}
	date, err = time.Parse(time.RFC3339, lines[0])
	if err != nil {
		return "", time.Time{}, fmt.Errorf("parsing date %q: %w", lines[0], err)
	}
	return lines[1], date, nil
}

// Upstream returns the remote and branch of the configured upstream for the
// worktree at path. Returns an error if no upstream is configured.
func Upstream(r CommandRunner, path string) (remote, branch string, err error) {
	out, err := r.Run("git", "-C", path, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		return "", "", fmt.Errorf("resolving upstream: %w", err)
	}
	ref := strings.TrimSpace(out)
	if ref == "" {
		return "", "", errors.New("empty upstream ref")
	}
	remote, branch, ok := strings.Cut(ref, "/")
	if !ok {
		return "", ref, nil
	}
	return remote, branch, nil
}

// AheadBehind returns the number of commits HEAD is ahead and behind upstream
// for the worktree at path. The upstream ref is used as-is (e.g. "origin/main").
func AheadBehind(r CommandRunner, path, upstream string) (ahead, behind int, err error) {
	out, err := r.Run("git", "-C", path, "rev-list", "--left-right", "--count", "HEAD..."+upstream)
	if err != nil {
		return 0, 0, fmt.Errorf("counting ahead/behind: %w", err)
	}
	parts := strings.Fields(strings.TrimSpace(out))
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected rev-list output: %q", out)
	}
	ahead, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("parsing ahead count: %w", err)
	}
	behind, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parsing behind count: %w", err)
	}
	return ahead, behind, nil
}
