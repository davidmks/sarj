// Package git wraps git CLI operations needed for worktree management.
package git

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/davidmks/sarj/internal/exec"
)

// RepoRoot returns the absolute path to the repository root.
func RepoRoot(r exec.Runner) (string, error) {
	out, err := r.Run("git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("finding repo root: %w", err)
	}
	return out, nil
}

// MainWorktree returns the path of the main (first) worktree.
// Git always lists the main worktree first in `git worktree list`.
func MainWorktree(r exec.Runner) (string, error) {
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
func DefaultBranch(r exec.Runner) string {
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
func Fetch(r exec.Runner, remote string) error {
	_, err := r.Run("git", "fetch", remote)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", remote, err)
	}
	return nil
}

// BranchExists checks if a local branch with the given name exists.
func BranchExists(r exec.Runner, name string) bool {
	_, err := r.Run("git", "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	return err == nil
}

// RemoteRefExists checks if a remote-tracking ref exists (e.g., "origin/main").
func RemoteRefExists(r exec.Runner, ref string) bool {
	_, err := r.Run("git", "show-ref", "--verify", "--quiet", "refs/remotes/"+ref)
	return err == nil
}

// CommitsBehind returns the number of commits local is behind remote.
// Both refs are used as-is (e.g., "main", "origin/main"). Returns 0 if the comparison fails.
func CommitsBehind(r exec.Runner, local, remote string) int {
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
