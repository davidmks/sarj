package cli

import (
	"path/filepath"

	"github.com/davidmks/sarj/internal/exec"
	"github.com/davidmks/sarj/internal/worktree"
	"github.com/spf13/cobra"
)

// completeWorktreeNames returns a completion function that suggests worktree
// directory names, excluding the main worktree (always the first entry from git).
func completeWorktreeNames(r exec.Runner) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		wts, err := worktree.List(r)
		if err != nil || len(wts) < 2 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var names []string
		for _, wt := range wts[1:] {
			names = append(names, filepath.Base(wt.Path))
		}

		return names, cobra.ShellCompDirectiveNoFileComp
	}
}
