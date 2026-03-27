package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/davidmks/sarj/internal/exec"
	"github.com/davidmks/sarj/internal/tmux"
	"github.com/davidmks/sarj/internal/worktree"
	"github.com/spf13/cobra"
)

func newDeleteCmd(r exec.Runner) *cobra.Command {
	var deleteBranch bool
	var keepBranch bool

	cmd := &cobra.Command{
		Use:   "delete [name]",
		Short: "Remove a worktree and optionally its branch",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := bufio.NewReader(cmd.InOrStdin())

			wts, err := worktree.List(r)
			if err != nil {
				return fmt.Errorf("listing worktrees: %w", err)
			}

			target, err := resolveWorktree(wts, args)
			if err != nil {
				return err
			}

			wt, name := target.wt, target.name

			if target.confirm && !promptYesNo(fmt.Sprintf("Delete worktree '%s'?", name), in) {
				return nil
			}

			if err := os.Chdir(worktree.MainPath(wts)); err != nil {
				return fmt.Errorf("changing to main worktree: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Removing worktree...\n")
			if err := worktree.Delete(r, worktree.DeleteOpts{
				Path:     wt.Path,
				Progress: os.Stderr,
			}); err != nil {
				return err
			}

			if !deleteBranch && !keepBranch {
				deleteBranch = promptYesNo(fmt.Sprintf("Delete branch '%s'?", wt.Branch), in)
			}

			branchStatus := "branch kept"
			if deleteBranch {
				if _, err := r.Run("git", "branch", "-D", wt.Branch); err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not delete branch: %v\n", err)
				} else {
					branchStatus = "branch deleted"
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Deleted worktree %s (%s)\n", wt.Branch, branchStatus) //nolint:errcheck

			// Session kill is last — it sends SIGHUP to the current process when
			// run from inside the target session. All cleanup is already done above,
			// so if the switch fails (no other session) the kill just closes the terminal.
			sessionName := tmux.SanitizeName(name)
			if tmux.IsInsideSession() && tmux.CurrentSessionName(r) == sessionName {
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

type deleteTarget struct {
	wt      *worktree.Worktree
	name    string
	confirm bool
}

// resolveWorktree resolves the target worktree from an explicit name or the
// current working directory. When inferred from cwd, confirm is true so the
// caller can prompt before proceeding.
func resolveWorktree(wts []worktree.Worktree, args []string) (deleteTarget, error) {
	if len(args) == 1 {
		name := args[0]
		wt := worktree.FindByName(wts, name)
		if wt == nil {
			return deleteTarget{}, fmt.Errorf("worktree %s not found", name)
		}
		return deleteTarget{wt: wt, name: name}, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return deleteTarget{}, fmt.Errorf("getting current directory: %w", err)
	}
	cwd, err = filepath.EvalSymlinks(cwd)
	if err != nil {
		return deleteTarget{}, fmt.Errorf("resolving current directory: %w", err)
	}
	wt := worktree.FindByPath(wts, cwd)
	if wt == nil {
		return deleteTarget{}, fmt.Errorf("current directory is not inside a worktree")
	}
	if wt.Path == worktree.MainPath(wts) {
		return deleteTarget{}, fmt.Errorf("cannot delete the main worktree")
	}
	return deleteTarget{wt: wt, name: filepath.Base(wt.Path), confirm: true}, nil
}

// promptYesNo asks a y/N question on stderr and reads from in. Default is no.
func promptYesNo(question string, in *bufio.Reader) bool {
	fmt.Fprintf(os.Stderr, "%s (y/N) ", question)
	answer, _ := in.ReadString('\n')
	return strings.TrimSpace(strings.ToLower(answer)) == "y"
}
