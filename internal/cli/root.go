// Package cli wires cobra commands to the internal packages.
package cli

import (
	"github.com/davidmks/sarj/internal/exec"
	"github.com/spf13/cobra"
)

// NewRootCmd builds the root cobra command with all subcommands registered.
func NewRootCmd(version string, r exec.Runner) *cobra.Command {
	root := &cobra.Command{
		Use:     "sarj",
		Short:   "Git worktree + tmux session manager",
		Version: version,
		// SilenceUsage prevents cobra from printing usage on every error —
		// we only want usage on --help, not on runtime failures.
		SilenceUsage: true,
	}

	root.AddCommand(
		newCreateCmd(r),
		newDeleteCmd(r),
		newListCmd(r),
		newInitCmd(r),
	)

	return root
}

// Execute builds the root command and runs it.
// This is the single entry point called from main.go.
func Execute(version string) error {
	return NewRootCmd(version, &exec.DefaultRunner{}).Execute()
}
