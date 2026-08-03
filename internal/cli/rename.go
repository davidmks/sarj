package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/davidmks/sarj/internal/exec"
	"github.com/davidmks/sarj/internal/git"
	"github.com/davidmks/sarj/internal/tmux"
	"github.com/davidmks/sarj/internal/worktree"
	"github.com/spf13/cobra"
)

func newRenameCmd(r exec.Runner) *cobra.Command {
	var keepPath, skipTmux bool

	cmd := &cobra.Command{
		Use:   "rename [name] <new-branch>",
		Short: "Rename a worktree's branch, directory, and tmux session",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			newBranch := args[len(args)-1]
			wts, err := worktree.List(ctx, r)
			if err != nil {
				return fmt.Errorf("listing worktrees: %w", err)
			}

			targets, err := resolveTargets(wts, args[:len(args)-1])
			if err != nil {
				return err
			}
			t := targets[0]

			newPath, err := renameDest(t, newBranch, keepPath)
			if err != nil {
				return err
			}
			if git.BranchExists(ctx, r, newBranch) {
				return fmt.Errorf("branch already exists: %s", newBranch)
			}

			// Read the upstream before the move, while the path is still valid.
			upstreamRemote, upstreamBranch, upstreamErr := git.Upstream(ctx, r, t.wt.Path)

			// git resolves `worktree move` against the process working
			// directory, which must not be the worktree being moved.
			if err := os.Chdir(worktree.MainPath(wts)); err != nil {
				return fmt.Errorf("changing to main worktree: %w", err)
			}

			if err := worktree.Rename(ctx, r, worktree.RenameOpts{
				OldBranch: t.wt.Branch,
				NewBranch: newBranch,
				OldPath:   t.wt.Path,
				NewPath:   newPath,
				Progress:  cmd.ErrOrStderr(),
			}); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Renamed worktree %s to %s\n", t.name, newBranch) //nolint:errcheck

			if !skipTmux {
				if err := tmux.RenameSession(ctx, r, t.wt.Branch, newBranch); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not rename tmux session: %v\n", err) //nolint:errcheck
				}
			}

			// `git branch -m` leaves the upstream pointing at the old remote
			// branch, which was not renamed along with the local one.
			if upstreamErr == nil {
				fmt.Fprintf(cmd.ErrOrStderr(), //nolint:errcheck
					"warning: branch still tracks %s/%s; the remote branch was not renamed\n",
					upstreamRemote, upstreamBranch)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&keepPath, "keep-path", false, "keep the worktree directory where it is")
	cmd.Flags().BoolVar(&skipTmux, "no-tmux", false, "skip renaming the tmux session")

	// Only the first positional names an existing worktree; the second is a
	// branch name that does not exist yet.
	complete := completeWorktreeNames(r)
	cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return complete(cmd, args, toComplete)
	}

	return cmd
}

// renameDest returns the directory the worktree should move to, or "" when it
// should stay put. Staying put covers both --keep-path and the case where the
// new branch name maps onto the directory name already in use, which happens
// whenever DirName collapses a slash the old name spelled as a dash.
func renameDest(t target, newBranch string, keepPath bool) (string, error) {
	if newBranch == t.wt.Branch {
		return "", fmt.Errorf("branch is already named %s", newBranch)
	}
	if keepPath {
		return "", nil
	}

	// Sibling of the current directory, so a rename never relocates a worktree
	// that lives outside the configured worktree base.
	newPath := filepath.Join(filepath.Dir(t.wt.Path), worktree.DirName(newBranch))
	if newPath == t.wt.Path {
		return "", nil
	}
	if _, err := os.Stat(newPath); err == nil {
		return "", fmt.Errorf("%w: %s", worktree.ErrWorktreeExists, newPath)
	}
	return newPath, nil
}
