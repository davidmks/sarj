// Package cli wires cobra commands to the internal packages.
package cli

import (
	"github.com/davidmks/sarj/internal/exec"
	"github.com/spf13/cobra"
)

// Execute builds the root command and runs it.
// This is the single entry point called from main.go.
func Execute(version string) error {
	root := &cobra.Command{
		Use:     "sarj",
		Short:   "Git worktree + tmux session manager",
		Version: version,
		// SilenceUsage prevents cobra from printing usage on every error —
		// we only want usage on --help, not on runtime failures.
		SilenceUsage: true,
	}

	r := &exec.DefaultRunner{}

	root.AddCommand(
		newCreateCmd(r),
		newDeleteCmd(r),
		newListCmd(r),
	)

	return root.Execute()
}
