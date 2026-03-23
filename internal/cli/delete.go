package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/davidmks/sarj/internal/config"
	"github.com/davidmks/sarj/internal/exec"
	"github.com/davidmks/sarj/internal/git"
	"github.com/davidmks/sarj/internal/tmux"
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

			sessionName := tmux.SanitizeName(opts.Name)
			fmt.Fprintf(os.Stderr, "Stopping tmux session...\n")
			if err := tmux.KillSession(r, sessionName); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not kill tmux session: %v\n", err)
			}

			fmt.Fprintf(os.Stderr, "Removing worktree...\n")
			if err := worktree.Delete(r, cfg, opts); err != nil {
				return err
			}

			if !opts.DeleteBranch && !opts.KeepBranch {
				opts.DeleteBranch = promptYesNo(fmt.Sprintf("Delete branch '%s'?", opts.Name))
			}

			branchStatus := "branch kept"
			if opts.DeleteBranch {
				if _, err := r.Run("git", "branch", "-D", opts.Name); err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not delete branch: %v\n", err)
				} else {
					branchStatus = "branch deleted"
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Deleted worktree %s (%s)\n", opts.Name, branchStatus) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().BoolVarP(&opts.DeleteBranch, "delete-branch", "D", false, "also delete the branch")
	cmd.Flags().BoolVar(&opts.KeepBranch, "keep-branch", false, "keep the branch (no prompt)")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "skip confirmation")

	cmd.ValidArgsFunction = completeWorktreeNames(r)

	return cmd
}

// promptYesNo asks a y/N question on stderr and reads from stdin. Default is no.
func promptYesNo(question string) bool {
	fmt.Fprintf(os.Stderr, "%s (y/N) ", question)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	return strings.TrimSpace(strings.ToLower(answer)) == "y"
}
