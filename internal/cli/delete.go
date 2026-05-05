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
	"github.com/davidmks/sarj/internal/status"
	"github.com/davidmks/sarj/internal/tmux"
	"github.com/davidmks/sarj/internal/worktree"
	"github.com/spf13/cobra"
)

func newDeleteCmd(r exec.Runner) *cobra.Command {
	var deleteBranch, keepBranch, yes bool
	var state string

	cmd := &cobra.Command{
		Use:   "delete [name...]",
		Short: "Remove one or more worktrees and optionally their branches",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			in := bufio.NewReader(cmd.InOrStdin())

			wts, err := worktree.List(r)
			if err != nil {
				return fmt.Errorf("listing worktrees: %w", err)
			}

			targets, err := resolveDeleteTargets(cmd, r, wts, args, state)
			if err != nil {
				return err
			}
			if len(targets) == 0 {
				return nil
			}

			// With -y and no explicit branch flag, keep branches by default.
			if yes && !deleteBranch && !keepBranch {
				keepBranch = true
			}

			// cwd-inferred deletes (zero-arg) prompt for confirmation unless -y.
			if len(targets) == 1 && targets[0].confirm && !yes {
				if !promptYesNo(fmt.Sprintf("Delete worktree '%s'?", targets[0].name), in) {
					return nil
				}
			}

			if err := os.Chdir(worktree.MainPath(wts)); err != nil {
				return fmt.Errorf("changing to main worktree: %w", err)
			}

			targets = deferCurrentSession(r, targets)

			opts := deleteOpts{deleteBranch: deleteBranch, keepBranch: keepBranch, yes: yes}
			var failed []string
			for _, t := range targets {
				if err := deleteOne(cmd, r, t, opts, in); err != nil {
					fmt.Fprintf(os.Stderr, "warning: %s: %v\n", t.name, err) //nolint:errcheck
					failed = append(failed, t.name)
				}
			}

			if len(failed) > 0 {
				return fmt.Errorf("%d of %d worktrees failed: %s",
					len(failed), len(targets), strings.Join(failed, ", "))
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&deleteBranch, "delete-branch", "D", false, "also delete the branch")
	cmd.Flags().BoolVar(&keepBranch, "keep-branch", false, "keep the branch (no prompt)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip prompts (defaults to keep-branch)")
	cmd.Flags().StringVar(&state, "state", "", "filter by state from [status] hook (comma-separated, e.g. merged,closed)")
	cmd.MarkFlagsMutuallyExclusive("delete-branch", "keep-branch")

	cmd.ValidArgsFunction = completeWorktreeNames(r)

	return cmd
}

type deleteTarget struct {
	wt      *worktree.Worktree
	name    string
	confirm bool
}

type deleteOpts struct {
	deleteBranch bool
	keepBranch   bool
	yes          bool
}

// resolveDeleteTargets resolves the candidate worktrees and applies the
// --state filter when set. Without --state it falls through to the existing
// arg-or-cwd behavior. With --state and no args, the candidate pool is all
// non-main worktrees rather than the cwd.
func resolveDeleteTargets(cmd *cobra.Command, r exec.Runner, wts []worktree.Worktree, args []string, state string) ([]deleteTarget, error) {
	if state == "" {
		return resolveTargets(wts, args)
	}

	cfg, err := loadDeleteConfig(r)
	if err != nil {
		return nil, err
	}
	if cfg == nil || cfg.Status.Command == "" {
		return nil, fmt.Errorf("--state requires [status] command in config")
	}

	wanted := parseStates(state)
	if len(wanted) == 0 {
		return nil, fmt.Errorf("--state requires at least one value")
	}

	pool, err := candidatePool(wts, args)
	if err != nil {
		return nil, err
	}
	if len(pool) == 0 {
		return nil, nil
	}

	items := make([]status.Item, len(pool))
	for i, t := range pool {
		items[i] = status.Item{Branch: t.wt.Branch, Path: t.wt.Path}
	}
	results := status.ProbeAll(cmd.Context(), r, cfg.Status.Command, items, 0)

	var filtered []deleteTarget
	for i, t := range pool {
		if wanted[results[i].State] {
			filtered = append(filtered, t)
		}
	}
	return filtered, nil
}

// candidatePool returns the worktrees the --state filter applies to: the
// named ones (with strict unknown-name failure) when args are given, or all
// non-main worktrees when args are empty.
func candidatePool(wts []worktree.Worktree, args []string) ([]deleteTarget, error) {
	if len(args) > 0 {
		return resolveNamed(wts, args)
	}
	mainPath := worktree.MainPath(wts)
	var pool []deleteTarget
	for i := range wts {
		wt := &wts[i]
		if wt.Path == mainPath {
			continue
		}
		pool = append(pool, deleteTarget{wt: wt, name: filepath.Base(wt.Path)})
	}
	return pool, nil
}

// parseStates splits a comma-separated --state value into a set, dropping
// empty entries from leading/trailing commas or runs of whitespace.
func parseStates(s string) map[string]bool {
	out := map[string]bool{}
	for part := range strings.SplitSeq(s, ",") {
		v := strings.TrimSpace(part)
		if v != "" {
			out[v] = true
		}
	}
	return out
}

// loadDeleteConfig loads the merged config from the main worktree's path.
// Returns nil when not inside a repo.
func loadDeleteConfig(r exec.Runner) (*config.Config, error) {
	mainPath, err := git.MainWorktree(r)
	if err != nil || mainPath == "" {
		return nil, nil //nolint:nilerr // not-a-repo: callers treat as no config
	}
	return config.Load(mainPath, filepath.Base(mainPath))
}

// resolveTargets resolves all delete targets up front so unknown names fail
// before any side effect. Zero args falls back to cwd inference.
func resolveTargets(wts []worktree.Worktree, args []string) ([]deleteTarget, error) {
	if len(args) == 0 {
		t, err := resolveFromCwd(wts)
		if err != nil {
			return nil, err
		}
		return []deleteTarget{t}, nil
	}
	return resolveNamed(wts, args)
}

func resolveNamed(wts []worktree.Worktree, args []string) ([]deleteTarget, error) {
	targets := make([]deleteTarget, 0, len(args))
	var unknown []string
	for _, name := range args {
		wt := worktree.FindByName(wts, name)
		if wt == nil {
			unknown = append(unknown, name)
			continue
		}
		targets = append(targets, deleteTarget{wt: wt, name: name})
	}
	if len(unknown) > 0 {
		return nil, fmt.Errorf("worktree not found: %s", strings.Join(unknown, ", "))
	}
	return targets, nil
}

func resolveFromCwd(wts []worktree.Worktree) (deleteTarget, error) {
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

// deferCurrentSession moves any target matching the current tmux session to the
// end of the queue so prior deletes complete before the SIGHUP-on-current-kill.
func deferCurrentSession(r exec.Runner, targets []deleteTarget) []deleteTarget {
	if !tmux.IsInsideSession() {
		return targets
	}
	current := tmux.CurrentSessionName(r)
	head := make([]deleteTarget, 0, len(targets))
	var tail []deleteTarget
	for _, t := range targets {
		if tmux.SanitizeName(t.name) == current {
			tail = append(tail, t)
		} else {
			head = append(head, t)
		}
	}
	return append(head, tail...)
}

func deleteOne(cmd *cobra.Command, r exec.Runner, t deleteTarget, opts deleteOpts, in *bufio.Reader) error {
	fmt.Fprintf(os.Stderr, "Removing worktree %s...\n", t.name) //nolint:errcheck
	if err := worktree.Delete(r, worktree.DeleteOpts{
		Path:     t.wt.Path,
		Progress: os.Stderr,
	}); err != nil {
		return err
	}

	deleteBranch := opts.deleteBranch
	if !deleteBranch && !opts.keepBranch && !opts.yes {
		deleteBranch = promptYesNo(fmt.Sprintf("Delete branch '%s'?", t.wt.Branch), in)
	}

	branchStatus := "branch kept"
	if deleteBranch {
		if _, err := r.Run("git", "branch", "-D", t.wt.Branch); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not delete branch: %v\n", err) //nolint:errcheck
		} else {
			branchStatus = "branch deleted"
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Deleted worktree %s (%s)\n", t.wt.Branch, branchStatus) //nolint:errcheck

	// Session kill is last — it sends SIGHUP to the current process when run
	// from inside the target session. With multi-arg, the current-session
	// target was deferred to the end so prior deletes already finished.
	sessionName := tmux.SanitizeName(t.name)
	if tmux.IsInsideSession() && tmux.CurrentSessionName(r) == sessionName {
		_ = tmux.SwitchToLastSession(r)
	}
	if err := tmux.KillSession(r, t.name); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not kill tmux session: %v\n", err) //nolint:errcheck
	}
	return nil
}

// promptYesNo asks a y/N question on stderr and reads from in. Default is no.
func promptYesNo(question string, in *bufio.Reader) bool {
	fmt.Fprintf(os.Stderr, "%s (y/N) ", question) //nolint:errcheck
	answer, _ := in.ReadString('\n')
	return strings.TrimSpace(strings.ToLower(answer)) == "y"
}
