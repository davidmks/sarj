package worktree

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/davidmks/sarj/internal/exec"
)

// List returns all worktrees by parsing `git worktree list --porcelain`.
func List(r exec.Runner) ([]Worktree, error) {
	out, err := r.Run("git", "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("listing worktrees: %w", err)
	}

	return parsePorcelain(out), nil
}

// FindBranch returns the branch checked out in the named worktree.
// Returns the worktree name itself if no match is found or the worktree is in
// detached HEAD state.
func FindBranch(wts []Worktree, wtBase, name string) string {
	wtPath := filepath.Join(wtBase, DirName(name))
	for _, wt := range wts {
		if wt.Path == wtPath && wt.Branch != "" {
			return wt.Branch
		}
	}
	return name
}

// parsePorcelain parses the output of `git worktree list --porcelain`.
// Each worktree entry is separated by a blank line.
func parsePorcelain(output string) []Worktree {
	var worktrees []Worktree
	var current Worktree

	for _, line := range strings.Split(output, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			current = Worktree{Path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "HEAD "):
			current.HEAD = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		case line == "bare":
			current.Bare = true
		case line == "":
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = Worktree{}
			}
		}
	}

	// Handle last entry (output may not end with blank line)
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees
}
