package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/davidmks/sarj/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRunner returns a preconfigured response for any command.
type fakeRunner struct {
	out string
	err error
}

func (f *fakeRunner) Run(_ string, _ ...string) (string, error) {
	return f.out, f.err
}

func (f *fakeRunner) RunInteractive(_ string, _ ...string) error {
	return f.err
}

func TestInitProject(t *testing.T) {
	dir := t.TempDir()
	r := &fakeRunner{out: dir}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"init"})
	require.NoError(t, cmd.Execute())

	configPath := filepath.Join(dir, ".sarj.toml")
	assert.Contains(t, buf.String(), configPath)
	assert.FileExists(t, configPath)

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "setup_command")
	assert.Contains(t, string(content), "symlinks")
}

func TestInitProject_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	r := &fakeRunner{out: dir}
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".sarj.toml"), []byte(""), 0o600))

	cmd := cli.NewRootCmd("test", r)
	cmd.SetArgs([]string{"init"})
	err := cmd.Execute()

	assert.ErrorContains(t, err, "config already exists")
}

func TestInitGlobal(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	r := &fakeRunner{}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"init", "--global"})
	require.NoError(t, cmd.Execute())

	configPath := filepath.Join(dir, "sarj", "config.toml")
	assert.Contains(t, buf.String(), configPath)
	assert.FileExists(t, configPath)

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "worktree_base")
	assert.Contains(t, string(content), "tmux.windows")
}

func TestInitGlobal_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	configDir := filepath.Join(dir, "sarj")
	require.NoError(t, os.MkdirAll(configDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(""), 0o600))

	r := &fakeRunner{}
	cmd := cli.NewRootCmd("test", r)
	cmd.SetArgs([]string{"init", "--global"})
	err := cmd.Execute()

	assert.ErrorContains(t, err, "config already exists")
}

func TestInitGlobal_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "deep", "nested")
	t.Setenv("XDG_CONFIG_HOME", nested)

	r := &fakeRunner{}
	cmd := cli.NewRootCmd("test", r)
	cmd.SetArgs([]string{"init", "--global"})
	require.NoError(t, cmd.Execute())

	assert.FileExists(t, filepath.Join(nested, "sarj", "config.toml"))
}
