package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/davidmks/sarj/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFile is a test helper that creates a file with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func TestDefaults(t *testing.T) {
	cfg := config.Defaults("myrepo")

	assert.Equal(t, "~/wt/myrepo", cfg.WorktreeBase)
	assert.Equal(t, "main", cfg.DefaultBranch)
	assert.True(t, cfg.AutoAttach)
	assert.True(t, cfg.Tmux.Enabled)
	assert.Len(t, cfg.Tmux.Windows, 1)
	assert.Equal(t, "terminal", cfg.Tmux.Windows[0].Name)
}

func TestLoadWithPaths_NoFiles(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "global.toml")
	project := filepath.Join(dir, "project.toml")

	cfg, err := config.LoadWithPaths(global, project, "myrepo")

	require.NoError(t, err)
	assert.Equal(t, "main", cfg.DefaultBranch)
	assert.Contains(t, cfg.WorktreeBase, "wt/myrepo")
}

func TestLoadWithPaths_GlobalOnly(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "global.toml")
	project := filepath.Join(dir, "project.toml")

	writeFile(t, global, `
worktree_base = "~/worktrees/{{.RepoName}}"
default_branch = "develop"
auto_attach = false
`)

	cfg, err := config.LoadWithPaths(global, project, "myrepo")

	require.NoError(t, err)
	assert.Contains(t, cfg.WorktreeBase, "worktrees/myrepo")
	assert.Equal(t, "develop", cfg.DefaultBranch)
	assert.False(t, cfg.AutoAttach)
}

func TestLoadWithPaths_ProjectOverridesGlobal(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "global.toml")
	project := filepath.Join(dir, "project.toml")

	writeFile(t, global, `default_branch = "main"`)
	writeFile(t, project, `
default_branch = "trunk"
setup_command = "make setup"
symlinks = [".env", ".env.test"]
`)

	cfg, err := config.LoadWithPaths(global, project, "myrepo")

	require.NoError(t, err)
	assert.Equal(t, "trunk", cfg.DefaultBranch)
	assert.Equal(t, "make setup", cfg.SetupCommand)
	assert.Equal(t, []string{".env", ".env.test"}, cfg.Symlinks)
}

func TestLoadWithPaths_TmuxWindowsFromGlobal(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "global.toml")
	project := filepath.Join(dir, "project.toml")

	writeFile(t, global, `
[[tmux.windows]]
name = "editor"
command = "nvim ."

[[tmux.windows]]
name = "claude"
command = "claude"
`)

	cfg, err := config.LoadWithPaths(global, project, "myrepo")

	require.NoError(t, err)
	assert.Len(t, cfg.Tmux.Windows, 2)
	assert.Equal(t, "editor", cfg.Tmux.Windows[0].Name)
	assert.Equal(t, "claude", cfg.Tmux.Windows[1].Name)
}

func TestLoadWithPaths_InvalidToml(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "global.toml")
	project := filepath.Join(dir, "project.toml")

	writeFile(t, global, `this is not valid toml [[[`)

	_, err := config.LoadWithPaths(global, project, "myrepo")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading global config")
}

func TestGlobalPath_XDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")
	assert.Equal(t, "/custom/config/sarj/config.toml", config.GlobalPath())
}

func TestExpandPath_Tilde(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "global.toml")
	project := filepath.Join(dir, "project.toml")

	writeFile(t, global, `worktree_base = "~/wt/{{.RepoName}}"`)

	cfg, err := config.LoadWithPaths(global, project, "myrepo")

	require.NoError(t, err)
	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, "wt", "myrepo"), cfg.WorktreeBase)
}
