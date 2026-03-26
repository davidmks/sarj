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
	var deleteBranch bool
	var keepBranch bool

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Remove a worktree and optionally its branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Resolve from the main worktree so config and paths are
			// correct even when invoked from inside a child worktree.
			mainWt, err := git.MainWorktree(r)
			if err != nil {
				return err
			}
			repoName := filepath.Base(mainWt)

			cfg, err := config.Load(mainWt, repoName)
			if err != nil {
				return err
			}

			if err := os.Chdir(mainWt); err != nil {
				return fmt.Errorf("changing to main worktree: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Removing worktree...\n")
			if err := worktree.Delete(r, worktree.DeleteOpts{
				WorktreeBase: cfg.WorktreeBase,
				Name:         name,
				Progress:     os.Stderr,
			}); err != nil {
				return err
			}

			if !deleteBranch && !keepBranch {
				deleteBranch = promptYesNo(fmt.Sprintf("Delete branch '%s'?", name))
			}

			branchStatus := "branch kept"
			if deleteBranch {
				if _, err := r.Run("git", "branch", "-D", name); err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not delete branch: %v\n", err)
				} else {
					branchStatus = "branch deleted"
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Deleted worktree %s (%s)\n", name, branchStatus) //nolint:errcheck

			// Session kill is last — it sends SIGHUP when run from inside the target session
			if tmux.IsInsideSession() && tmux.CurrentSessionName(r) == tmux.SanitizeName(name) {
				_ = tmux.SwitchToLastSession(r)
			}
			if err := tmux.KillSession(r, name); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not kill tmux session: %v\n", err)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&deleteBranch, "delete-branch", "D", false, "also delete the branch")
	cmd.Flags().BoolVar(&keepBranch, "keep-branch", false, "keep the branch (no prompt)")
	cmd.MarkFlagsMutuallyExclusive("delete-branch", "keep-branch")

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
