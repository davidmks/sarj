package cli

import (
	"fmt"
	"path/filepath"
	"text/tabwriter"

	"github.com/davidmks/sarj/internal/exec"
	"github.com/davidmks/sarj/internal/git"
	"github.com/davidmks/sarj/internal/tmux"
	"github.com/davidmks/sarj/internal/worktree"
	"github.com/spf13/cobra"
)

func newListCmd(r exec.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active worktrees",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			wts, err := worktree.List(r)
			if err != nil {
				return err
			}

			mainWT, _ := git.MainWorktree(r)

			sessions, _ := tmux.ListSessions(r)
			sessionSet := make(map[string]bool, len(sessions))
			for _, s := range sessions {
				sessionSet[s] = true
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tBRANCH\tTMUX") //nolint:errcheck

			for _, wt := range wts {
				if wt.Path == mainWT {
					continue
				}

				name := filepath.Base(wt.Path)
				sessionName := tmux.SanitizeName(name)

				tmuxStatus := "-"
				if sessionSet[sessionName] {
					tmuxStatus = "active"
				}

				fmt.Fprintf(w, "%s\t%s\t%s\n", name, wt.Branch, tmuxStatus) //nolint:errcheck
			}

			return w.Flush()
		},
	}

	return cmd
}
