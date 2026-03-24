// Package worktree orchestrates git worktree lifecycle operations.
package worktree

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/davidmks/sarj/internal/config"
	"github.com/davidmks/sarj/internal/exec"
	"github.com/davidmks/sarj/internal/git"
)

// ErrWorktreeExists is returned when a worktree directory already exists.
var ErrWorktreeExists = errors.New("worktree already exists")

// Worktree represents a single git worktree entry.
type Worktree struct {
	Path   string
	Branch string
	HEAD   string
	Bare   bool
}

// CreateOpts holds options for creating a worktree.
type CreateOpts struct {
	Name         string
	Base         string
	SkipSetup    bool
	SkipSymlinks bool
	Progress     io.Writer
}

// Create creates a new worktree with optional symlinks and setup command.
func Create(r exec.Runner, cfg *config.Config, opts CreateOpts) (*Worktree, error) {
	if opts.Name == "" {
		opts.Name = GenerateName()
	}
	if opts.Base == "" {
		opts.Base = cfg.DefaultBranch
	}

	w := progressWriter(opts.Progress)
	dirName := strings.ReplaceAll(opts.Name, "/", "-")
	wtPath := filepath.Join(cfg.WorktreeBase, dirName)

	if _, err := os.Stat(wtPath); err == nil {
		return nil, fmt.Errorf("%w: %s", ErrWorktreeExists, wtPath)
	}

	if err := os.MkdirAll(cfg.WorktreeBase, 0o750); err != nil {
		return nil, fmt.Errorf("creating worktree base dir: %w", err)
	}

	progress(w, "Fetching origin...\n")
	if err := git.Fetch(r, "origin"); err != nil {
		progress(w, "warning: fetch failed, continuing with local state: %v\n", err)
	}

	progress(w, "Creating worktree %s at %s\n", opts.Name, wtPath)
	branchCreated, err := addWorktree(r, wtPath, opts.Name, opts.Base)
	if err != nil {
		return nil, err
	}

	success := false
	defer func() {
		if !success {
			rollback(r, wtPath, opts.Name, branchCreated)
		}
	}()

	if !opts.SkipSymlinks && len(cfg.Symlinks) > 0 {
		if err := setupSymlinks(r, w, wtPath, cfg.Symlinks); err != nil {
			return nil, err
		}
	}

	if !opts.SkipSetup && cfg.SetupCommand != "" {
		progress(w, "Running setup command...\n")
		cmd := fmt.Sprintf("cd %q && %s", wtPath, cfg.SetupCommand)
		if err := r.RunInteractive("sh", "-c", cmd); err != nil {
			return nil, fmt.Errorf("setup command failed: %w", err)
		}
	}

	success = true
	return &Worktree{
		Path:   wtPath,
		Branch: opts.Name,
	}, nil
}

func setupSymlinks(r exec.Runner, w io.Writer, wtPath string, symlinks []string) error {
	progress(w, "Symlinking %d files...\n", len(symlinks))
	mainRepo, err := git.MainWorktree(r)
	if err != nil {
		return fmt.Errorf("finding main worktree for symlinks: %w", err)
	}
	if err := CreateSymlinks(mainRepo, wtPath, symlinks); err != nil {
		return fmt.Errorf("creating symlinks: %w", err)
	}
	return nil
}

// DeleteOpts holds options for deleting a worktree.
type DeleteOpts struct {
	WorktreeBase string
	Name         string
	Progress     io.Writer
}

// Delete removes a worktree and prunes stale references.
// Branch deletion is handled by the CLI layer (may require user prompt).
func Delete(r exec.Runner, opts DeleteOpts) error {
	w := progressWriter(opts.Progress)
	dirName := strings.ReplaceAll(opts.Name, "/", "-")
	wtPath := filepath.Join(opts.WorktreeBase, dirName)

	if _, err := r.Run("git", "worktree", "remove", "--force", wtPath); err != nil {
		return fmt.Errorf("removing worktree %s: %w", opts.Name, err)
	}

	if _, err := r.Run("git", "worktree", "prune"); err != nil {
		progress(w, "warning: worktree prune failed: %v\n", err)
	}

	return nil
}

func progressWriter(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}

func progress(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, format, args...) //nolint:errcheck
}

// addWorktree creates the git worktree, reusing an existing branch or creating a new one.
// Returns whether a new branch was created (for rollback decisions).
func addWorktree(r exec.Runner, wtPath, name, base string) (bool, error) {
	if git.BranchExists(r, name) {
		if _, err := r.Run("git", "worktree", "add", wtPath, name); err != nil {
			return false, fmt.Errorf("adding worktree for existing branch: %w", err)
		}
		return false, nil
	}

	if _, err := r.Run("git", "worktree", "add", "-b", name, wtPath, base); err != nil {
		return false, fmt.Errorf("adding worktree with new branch: %w", err)
	}
	return true, nil
}

func rollback(r exec.Runner, wtPath, branch string, branchCreated bool) {
	r.Run("git", "worktree", "remove", "--force", wtPath) //nolint:errcheck
	if branchCreated {
		r.Run("git", "branch", "-D", branch) //nolint:errcheck
	}
}
