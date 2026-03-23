package cli

import (
	"fmt"
	"path/filepath"

	"github.com/davidmks/sarj/internal/config"
	"github.com/davidmks/sarj/internal/exec"
	"github.com/davidmks/sarj/internal/git"
	"github.com/davidmks/sarj/internal/worktree"
	"github.com/spf13/cobra"
)

func newCreateCmd(r exec.Runner) *cobra.Command {
	var opts worktree.CreateOpts

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

			fmt.Fprintf(cmd.OutOrStdout(), "Created worktree %s at %s\n", wt.Branch, wt.Path)
			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.Base, "base", "b", "", "base branch (default: auto-detect)")
	cmd.Flags().BoolVar(&opts.SkipSetup, "no-setup", false, "skip setup command")
	cmd.Flags().BoolVar(&opts.SkipSymlinks, "no-symlinks", false, "skip symlinking")

	return cmd
}
