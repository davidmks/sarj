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

// MainPath returns the path of the main (first) worktree from a pre-fetched
// list. Git always lists the main worktree first.
func MainPath(wts []Worktree) string {
	if len(wts) > 0 {
		return wts[0].Path
	}
	return ""
}

// FindByName returns the worktree whose directory basename matches the given
// name. Returns nil if no match is found.
func FindByName(wts []Worktree, name string) *Worktree {
	dirName := DirName(name)
	for i := range wts {
		if filepath.Base(wts[i].Path) == dirName {
			return &wts[i]
		}
	}
	return nil
}

// FindByPath returns the worktree that contains the given absolute path.
// A path is "inside" a worktree if it equals or is a subdirectory of the
// worktree root. Returns nil if no match is found.
func FindByPath(wts []Worktree, path string) *Worktree {
	path = filepath.Clean(path)
	for i := range wts {
		wtPath := filepath.Clean(wts[i].Path)
		rel, err := filepath.Rel(wtPath, path)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return &wts[i]
		}
	}
	return nil
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
