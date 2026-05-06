package cli

import (
	"context"
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
	var cmdArgs string

	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a worktree with optional tmux session",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if len(args) > 0 {
				opts.Name = args[0]
			}

			mainWt, err := git.MainWorktree(ctx, r)
			if err != nil {
				return err
			}
			repoName := filepath.Base(mainWt)

			cfg, err := config.Load(mainWt, repoName)
			if err != nil {
				return err
			}

			opts.Progress = os.Stderr

			if !cmd.Flags().Changed("no-setup") && !cfg.IsAutoSetup() {
				opts.SkipSetup = true
			}

			wt, err := worktree.Create(ctx, r, cfg, opts)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created worktree %s\n", wt.Branch) //nolint:errcheck

			if !skipTmux && cfg.Tmux.Enabled {
				// Clear the {{.SetupCommand}} placeholder when the user
				// explicitly passes --no-setup so a tmux pane referencing it
				// doesn't run the setup the user just opted out of.
				setupCmd := cfg.SetupCommand
				if cmd.Flags().Changed("no-setup") && opts.SkipSetup {
					setupCmd = ""
				}
				if err := createTmuxSession(ctx, r, cfg, wt, skipAttach, cmdArgs, setupCmd); err != nil {
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
	cmd.Flags().StringVar(&cmdArgs, "args", "", "arguments to pass to commands containing {{.Args}}")

	return cmd
}

// createTmuxSession creates a tmux session for the worktree and optionally connects.
func createTmuxSession(ctx context.Context, r exec.Runner, cfg *config.Config, wt *worktree.Worktree, skipAttach bool, cmdArgs, setupCmd string) error {
	if !tmux.IsInstalled(ctx, r) {
		fmt.Fprintln(os.Stderr, "warning: tmux not found, skipping session creation")
		return nil
	}

	if err := tmux.CreateSession(ctx, r, wt.Branch, wt.Path, cfg.Tmux.Windows, cmdArgs, setupCmd); err != nil {
		return err
	}

	sessionName := tmux.SanitizeName(wt.Branch)
	fmt.Fprintf(os.Stderr, "Created tmux session %s\n", sessionName)

	if skipAttach || !cfg.AutoAttach {
		return nil
	}

	return tmux.Connect(ctx, r, wt.Branch)
}
