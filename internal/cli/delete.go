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

func newDeleteCmd(r exec.Runner) *cobra.Command {
	var opts worktree.DeleteOpts

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Remove a worktree and optionally its branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Name = args[0]

			repoRoot, err := git.RepoRoot(r)
			if err != nil {
				return err
			}
			repoName := filepath.Base(repoRoot)

			cfg, err := config.Load(repoRoot, repoName)
			if err != nil {
				return err
			}

			if err := worktree.Delete(r, cfg, opts); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Deleted worktree %s\n", opts.Name)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&opts.DeleteBranch, "delete-branch", "D", false, "also delete the branch")
	cmd.Flags().BoolVar(&opts.KeepBranch, "keep-branch", false, "keep the branch (no prompt)")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "skip confirmation")

	return cmd
}
