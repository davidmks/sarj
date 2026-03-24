package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/davidmks/sarj/internal/config"
	"github.com/davidmks/sarj/internal/exec"
	"github.com/davidmks/sarj/internal/git"
	"github.com/spf13/cobra"
)

const globalConfigTemplate = `# sarj global configuration
# See https://github.com/davidmks/sarj for documentation
#
# All values below show the built-in defaults. Uncomment and modify as needed.

# Base directory for worktrees. {{.RepoName}} expands to the repo directory name.
# worktree_base = "~/wt/{{.RepoName}}"

# Default branch to base new worktrees on (per-project .sarj.toml overrides this).
# default_branch = "main"

# Automatically attach to tmux session after creation.
# auto_attach = true

# [tmux]
# enabled = true
#
# [[tmux.windows]]
# name = "terminal"
# command = ""
#
# [[tmux.windows]]
# name = "editor"
# command = "nvim ."
# env_file = ".env.test"
#
# [[tmux.windows]]
# name = "script"
# command = ""
# env = { UV_ENV_FILE = ".env" }
#
# # Environment variables: use env_file to source a file (all variables are
# # exported) or env to set individual variables. Both can be combined — the
# # file is sourced first, then individual vars are exported. Panes inherit
# # environment from their parent window.
#
# # Windows can have panes for side-by-side layouts.
# [[tmux.windows]]
# name = "dev"
#
# [[tmux.windows.panes]]
# command = "make dev"
# size = 70
#
# [[tmux.windows.panes]]
# command = "make test-watch"
# split = "horizontal"
`

const projectConfigTemplate = `# sarj per-project configuration
# Commit this file to your repo so all contributors share the same settings.
# See https://github.com/davidmks/sarj for documentation

# Override the default branch (e.g., if your project uses "trunk" or "develop").
# default_branch = "main"

# Command to run after creating a worktree (e.g., install dependencies).
# setup_command = "make setup"

# Files and directories to symlink from the main worktree into new ones.
# symlinks = [
#     ".env",
#     ".env.secrets",
#     "ssl",
#     ".claude/settings.local.json",
# ]
`

func newInitCmd(r exec.Runner) *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate a config file with commented defaults",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if global {
				return initGlobal(cmd)
			}
			return initProject(cmd, r)
		},
	}

	cmd.Flags().BoolVar(&global, "global", false, "generate global config at ~/.config/sarj/config.toml")

	return cmd
}

func initGlobal(cmd *cobra.Command) error {
	path, err := config.GlobalPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config already exists: %s", path)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(globalConfigTemplate), 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", path) //nolint:errcheck
	return nil
}

func initProject(cmd *cobra.Command, r exec.Runner) error {
	repoRoot, err := git.RepoRoot(r)
	if err != nil {
		return err
	}

	path := config.ProjectPath(repoRoot)

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config already exists: %s", path)
	}

	if err := os.WriteFile(path, []byte(projectConfigTemplate), 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", path) //nolint:errcheck
	return nil
}
