package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/davidmks/sarj/internal/config"
	"github.com/davidmks/sarj/internal/exec"
	"github.com/davidmks/sarj/internal/git"
	"github.com/davidmks/sarj/internal/tmux"
	"github.com/davidmks/sarj/internal/worktree"
	"github.com/spf13/cobra"
)

func newCreateCmd(r exec.Runner) *cobra.Command {
	var opts worktree.CreateOpts
	var skipTmux bool
	var skipAttach bool

	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a worktree with optional tmux session",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Name = args[0]
			}

			repoRoot, err := git.RepoRoot(r)
			if err != nil {
				return err
			}
			repoName := filepath.Base(repoRoot)

			cfg, err := config.Load(repoRoot, repoName)
			if err != nil {
				return err
			}

			wt, err := worktree.Create(r, cfg, opts)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created worktree %s at %s\n", wt.Branch, wt.Path) //nolint:errcheck

			if !skipTmux && cfg.Tmux.Enabled {
				if err := createTmuxSession(r, cfg, wt, skipAttach); err != nil {
					fmt.Fprintf(os.Stderr, "warning: tmux session failed: %v\n", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.Base, "base", "b", "", "base branch (default: auto-detect)")
	cmd.Flags().BoolVar(&opts.SkipSetup, "no-setup", false, "skip setup command")
	cmd.Flags().BoolVar(&opts.SkipSymlinks, "no-symlinks", false, "skip symlinking")
	cmd.Flags().BoolVar(&skipTmux, "no-tmux", false, "skip tmux session creation")
	cmd.Flags().BoolVar(&skipAttach, "no-attach", false, "create tmux session but don't attach")

	return cmd
}

// createTmuxSession creates a tmux session for the worktree and optionally connects.
func createTmuxSession(r exec.Runner, cfg *config.Config, wt *worktree.Worktree, skipAttach bool) error {
	if !tmux.IsInstalled(r) {
		fmt.Fprintln(os.Stderr, "warning: tmux not found, skipping session creation")
		return nil
	}

	sessionName := tmux.SanitizeName(wt.Branch)

	if err := tmux.CreateSession(r, sessionName, wt.Path, cfg.Tmux.Windows); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Created tmux session %s\n", sessionName)

	if skipAttach || !cfg.AutoAttach {
		return nil
	}

	return tmux.Connect(r, sessionName)
}
