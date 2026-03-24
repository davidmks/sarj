package cli

import (
	"path/filepath"

	"github.com/davidmks/sarj/internal/exec"
	"github.com/davidmks/sarj/internal/git"
	"github.com/davidmks/sarj/internal/worktree"
	"github.com/spf13/cobra"
)

// completeWorktreeNames returns a completion function that suggests worktree
// directory names, excluding the main worktree.
func completeWorktreeNames(r exec.Runner) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		wts, err := worktree.List(r)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		mainWT, _ := git.MainWorktree(r)

		var names []string
		for _, wt := range wts {
			if wt.Path == mainWT {
				continue
			}
			names = append(names, filepath.Base(wt.Path))
		}

		return names, cobra.ShellCompDirectiveNoFileComp
	}
}
