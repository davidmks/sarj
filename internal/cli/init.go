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

# Skip the setup command during sarj create by default — useful when
# running setup yourself via a dedicated tmux window. Defaults to true
# (run setup); the --no-setup flag still applies on top.
# auto_setup = false

# Files and directories to symlink from the main worktree into new ones.
# symlinks = [
#     ".env",
#     ".env.secrets",
#     "ssl",
#     ".claude/settings.local.json",
# ]

# Tmux windows for this project (overrides global windows).
# [[tmux.windows]]
# name = "dev"
# command = "make dev"
`

const localConfigTemplate = `# sarj local configuration (per-user, per-project)
# DO NOT commit this file — add .sarj.local.toml to .gitignore.
# Sections defined here override the corresponding section from .sarj.toml.
# See https://github.com/davidmks/sarj for documentation

# Override the setup command for your local environment.
# setup_command = "make setup-local"

# Override whether sarj create runs the setup command automatically.
# auto_setup = false

# Override symlinks — replaces the project symlinks entirely.
# symlinks = [".env"]

# Override tmux windows — replaces the global windows entirely.
# [tmux]
# enabled = true
#
# [[tmux.windows]]
# name = "server"
# command = "make dev"
`

func newInitCmd(r exec.Runner) *cobra.Command {
	var global, local bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate a config file with commented defaults",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if global {
				return initGlobal(cmd)
			}
			if local {
				return initLocal(cmd, r)
			}
			return initProject(cmd, r)
		},
	}

	cmd.Flags().BoolVar(&global, "global", false, "generate global config at ~/.config/sarj/config.toml")
	cmd.Flags().BoolVar(&local, "local", false, "generate local config at .sarj.local.toml (gitignored, per-user)")
	cmd.MarkFlagsMutuallyExclusive("global", "local")

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

func initLocal(cmd *cobra.Command, r exec.Runner) error {
	repoRoot, err := git.RepoRoot(r)
	if err != nil {
		return err
	}

	path := config.LocalPath(repoRoot)

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config already exists: %s", path)
	}

	if err := os.WriteFile(path, []byte(localConfigTemplate), 0o600); err != nil {
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
