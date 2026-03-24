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

type testPaths struct {
	global  string
	project string
	local   string
}

func newTestPaths(t *testing.T) testPaths {
	t.Helper()
	dir := t.TempDir()
	return testPaths{
		global:  filepath.Join(dir, "global.toml"),
		project: filepath.Join(dir, "project.toml"),
		local:   filepath.Join(dir, "local.toml"),
	}
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
	p := newTestPaths(t)

	cfg, err := config.LoadWithPaths(p.global, p.project, p.local, "myrepo")

	require.NoError(t, err)
	assert.Equal(t, "main", cfg.DefaultBranch)
	assert.Contains(t, cfg.WorktreeBase, "wt/myrepo")
}

func TestLoadWithPaths_GlobalOnly(t *testing.T) {
	p := newTestPaths(t)

	writeFile(t, p.global, `
worktree_base = "~/worktrees/{{.RepoName}}"
default_branch = "develop"
auto_attach = false
`)

	cfg, err := config.LoadWithPaths(p.global, p.project, p.local, "myrepo")

	require.NoError(t, err)
	assert.Contains(t, cfg.WorktreeBase, "worktrees/myrepo")
	assert.Equal(t, "develop", cfg.DefaultBranch)
	assert.False(t, cfg.AutoAttach)
}

func TestLoadWithPaths_ProjectOverridesGlobal(t *testing.T) {
	p := newTestPaths(t)

	writeFile(t, p.global, `default_branch = "main"`)
	writeFile(t, p.project, `
default_branch = "trunk"
setup_command = "make setup"
symlinks = [".env", ".env.test"]
`)

	cfg, err := config.LoadWithPaths(p.global, p.project, p.local, "myrepo")

	require.NoError(t, err)
	assert.Equal(t, "trunk", cfg.DefaultBranch)
	assert.Equal(t, "make setup", cfg.SetupCommand)
	assert.Equal(t, []string{".env", ".env.test"}, cfg.Symlinks)
}

func TestLoadWithPaths_TmuxWindowsFromGlobal(t *testing.T) {
	p := newTestPaths(t)

	writeFile(t, p.global, `
[[tmux.windows]]
name = "editor"
command = "nvim ."

[[tmux.windows]]
name = "claude"
command = "claude"
`)

	cfg, err := config.LoadWithPaths(p.global, p.project, p.local, "myrepo")

	require.NoError(t, err)
	assert.Len(t, cfg.Tmux.Windows, 2)
	assert.Equal(t, "editor", cfg.Tmux.Windows[0].Name)
	assert.Equal(t, "claude", cfg.Tmux.Windows[1].Name)
}

func TestLoadWithPaths_ProjectOverridesTmuxWindows(t *testing.T) {
	p := newTestPaths(t)

	writeFile(t, p.global, `
[[tmux.windows]]
name = "editor"
command = "nvim ."
`)
	writeFile(t, p.project, `
[[tmux.windows]]
name = "dev"
command = "make dev"
`)

	cfg, err := config.LoadWithPaths(p.global, p.project, p.local, "myrepo")

	require.NoError(t, err)
	assert.Len(t, cfg.Tmux.Windows, 1)
	assert.Equal(t, "dev", cfg.Tmux.Windows[0].Name)
}

func TestLoadWithPaths_InvalidToml(t *testing.T) {
	p := newTestPaths(t)

	writeFile(t, p.global, `this is not valid toml [[[`)

	_, err := config.LoadWithPaths(p.global, p.project, p.local, "myrepo")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading global config")
}

func TestGlobalPath_XDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")
	path, err := config.GlobalPath()
	require.NoError(t, err)
	assert.Equal(t, "/custom/config/sarj/config.toml", path)
}

func TestExpandPath_Tilde(t *testing.T) {
	p := newTestPaths(t)

	writeFile(t, p.global, `worktree_base = "~/wt/{{.RepoName}}"`)

	cfg, err := config.LoadWithPaths(p.global, p.project, p.local, "myrepo")

	require.NoError(t, err)
	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, "wt", "myrepo"), cfg.WorktreeBase)
}

func TestLoadWithPaths_LocalOverridesSymlinks(t *testing.T) {
	p := newTestPaths(t)

	writeFile(t, p.project, `symlinks = [".env", ".env.test"]`)
	writeFile(t, p.local, `symlinks = [".env.local"]`)

	cfg, err := config.LoadWithPaths(p.global, p.project, p.local, "myrepo")

	require.NoError(t, err)
	assert.Equal(t, []string{".env.local"}, cfg.Symlinks)
}

func TestLoadWithPaths_LocalOverridesTmux(t *testing.T) {
	p := newTestPaths(t)

	writeFile(t, p.global, `
[[tmux.windows]]
name = "editor"
command = "nvim ."
`)
	writeFile(t, p.local, `
[[tmux.windows]]
name = "code"
command = "code ."
`)

	cfg, err := config.LoadWithPaths(p.global, p.project, p.local, "myrepo")

	require.NoError(t, err)
	assert.Len(t, cfg.Tmux.Windows, 1)
	assert.Equal(t, "code", cfg.Tmux.Windows[0].Name)
}

func TestLoadWithPaths_LocalOverridesSetupCommand(t *testing.T) {
	p := newTestPaths(t)

	writeFile(t, p.project, `
setup_command = "make setup"
symlinks = [".env"]
`)
	writeFile(t, p.local, `setup_command = "make setup-local"`)

	cfg, err := config.LoadWithPaths(p.global, p.project, p.local, "myrepo")

	require.NoError(t, err)
	assert.Equal(t, "make setup-local", cfg.SetupCommand)
	assert.Equal(t, []string{".env"}, cfg.Symlinks, "symlinks should fall through from project")
}

func TestLoadWithPaths_LocalNoFile(t *testing.T) {
	p := newTestPaths(t)

	writeFile(t, p.global, `default_branch = "develop"`)
	writeFile(t, p.project, `setup_command = "make setup"`)

	cfg, err := config.LoadWithPaths(p.global, p.project, p.local, "myrepo")

	require.NoError(t, err)
	assert.Equal(t, "develop", cfg.DefaultBranch)
	assert.Equal(t, "make setup", cfg.SetupCommand)
}

func TestLoadWithPaths_InvalidLocalToml(t *testing.T) {
	p := newTestPaths(t)

	writeFile(t, p.local, `not valid [[[`)

	_, err := config.LoadWithPaths(p.global, p.project, p.local, "myrepo")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading local config")
}
