// Package worktree orchestrates git worktree lifecycle operations.
package worktree

import (
	"errors"
	"fmt"
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
}

// DeleteOpts holds options for deleting a worktree.
type DeleteOpts struct {
	Name         string
	DeleteBranch bool
	KeepBranch   bool
	Force        bool
}

// Create creates a new worktree with optional symlinks and setup command.
func Create(r exec.Runner, cfg *config.Config, opts CreateOpts) (*Worktree, error) {
	if opts.Name == "" {
		opts.Name = GenerateName()
	}
	if opts.Base == "" {
		opts.Base = cfg.DefaultBranch
	}

	dirName := strings.ReplaceAll(opts.Name, "/", "-")
	wtPath := filepath.Join(cfg.WorktreeBase, dirName)

	if _, err := os.Stat(wtPath); err == nil {
		return nil, fmt.Errorf("%w: %s", ErrWorktreeExists, wtPath)
	}

	if err := os.MkdirAll(cfg.WorktreeBase, 0o755); err != nil {
		return nil, fmt.Errorf("creating worktree base dir: %w", err)
	}

	if err := git.Fetch(r, "origin"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: fetch failed, continuing with local state: %v\n", err)
	}

	branchCreated := false
	success := false
	defer func() {
		if !success {
			rollback(r, wtPath, opts.Name, branchCreated)
		}
	}()

	if git.BranchExists(r, opts.Name) {
		if _, err := r.Run("git", "worktree", "add", wtPath, opts.Name); err != nil {
			return nil, fmt.Errorf("adding worktree for existing branch: %w", err)
		}
	} else {
		if _, err := r.Run("git", "worktree", "add", "-b", opts.Name, wtPath, opts.Base); err != nil {
			return nil, fmt.Errorf("adding worktree with new branch: %w", err)
		}
		branchCreated = true
	}

	if !opts.SkipSymlinks && len(cfg.Symlinks) > 0 {
		mainRepo, err := git.MainWorktree(r)
		if err != nil {
			return nil, fmt.Errorf("finding main worktree for symlinks: %w", err)
		}
		if err := CreateSymlinks(mainRepo, wtPath, cfg.Symlinks); err != nil {
			return nil, fmt.Errorf("creating symlinks: %w", err)
		}
	}

	if !opts.SkipSetup && cfg.SetupCommand != "" {
		cmd := fmt.Sprintf("cd %q && %s", wtPath, cfg.SetupCommand)
		if _, err := r.Run("sh", "-c", cmd); err != nil {
			return nil, fmt.Errorf("setup command failed: %w", err)
		}
	}

	success = true
	return &Worktree{
		Path:   wtPath,
		Branch: opts.Name,
	}, nil
}

// Delete removes a worktree and optionally its branch.
func Delete(r exec.Runner, cfg *config.Config, opts DeleteOpts) error {
	dirName := strings.ReplaceAll(opts.Name, "/", "-")
	wtPath := filepath.Join(cfg.WorktreeBase, dirName)

	if _, err := r.Run("git", "worktree", "remove", "--force", wtPath); err != nil {
		return fmt.Errorf("removing worktree %s: %w", opts.Name, err)
	}

	if opts.DeleteBranch {
		if _, err := r.Run("git", "branch", "-D", opts.Name); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not delete branch %s: %v\n", opts.Name, err)
		}
	}

	if _, err := r.Run("git", "worktree", "prune"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: worktree prune failed: %v\n", err)
	}

	return nil
}

func rollback(r exec.Runner, wtPath, branch string, branchCreated bool) {
	r.Run("git", "worktree", "remove", "--force", wtPath) //nolint:errcheck
	if branchCreated {
		r.Run("git", "branch", "-D", branch) //nolint:errcheck
	}
}
