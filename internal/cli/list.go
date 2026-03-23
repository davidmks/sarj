package cli

import (
	"fmt"
	"path/filepath"
	"text/tabwriter"

	"github.com/davidmks/sarj/internal/exec"
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

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tBRANCH\tPATH") //nolint:errcheck

			for _, wt := range wts {
				name := filepath.Base(wt.Path)
				fmt.Fprintf(w, "%s\t%s\t%s\n", name, wt.Branch, wt.Path) //nolint:errcheck
			}

			return w.Flush()
		},
	}

	return cmd
}
