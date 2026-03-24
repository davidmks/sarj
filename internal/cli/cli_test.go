package cli_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidmks/sarj/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitProject(t *testing.T) {
	dir := t.TempDir()
	r := &fakeRunner{responses: map[string]response{
		"git rev-parse": {out: dir},
	}}

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
	r := &fakeRunner{responses: map[string]response{
		"git rev-parse": {out: dir},
	}}
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

func TestInitLocal(t *testing.T) {
	dir := t.TempDir()
	r := &fakeRunner{responses: map[string]response{
		"git rev-parse": {out: dir},
	}}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"init", "--local"})
	require.NoError(t, cmd.Execute())

	configPath := filepath.Join(dir, ".sarj.local.toml")
	assert.Contains(t, buf.String(), configPath)
	assert.FileExists(t, configPath)

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "DO NOT commit")
	assert.Contains(t, string(content), "setup_command")
}

func TestInitLocal_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	r := &fakeRunner{responses: map[string]response{
		"git rev-parse": {out: dir},
	}}
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".sarj.local.toml"), []byte(""), 0o600))

	cmd := cli.NewRootCmd("test", r)
	cmd.SetArgs([]string{"init", "--local"})
	err := cmd.Execute()

	assert.ErrorContains(t, err, "config already exists")
}

func TestInitGlobalAndLocal_MutuallyExclusive(t *testing.T) {
	r := &fakeRunner{}
	cmd := cli.NewRootCmd("test", r)
	cmd.SetArgs([]string{"init", "--global", "--local"})
	err := cmd.Execute()

	require.Error(t, err)
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

func TestListCmd(t *testing.T) {
	isolateConfig(t)
	porcelain := "worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree /wt/my-feature\nHEAD def\nbranch refs/heads/my-feature\n\n"

	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"git worktree list":             {out: porcelain},
		"tmux list-sessions":            {out: "my-feature"},
	}}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"list"})

	require.NoError(t, cmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "my-feature")
	assert.Contains(t, out, "active")
}

func TestListCmd_Empty(t *testing.T) {
	isolateConfig(t)
	porcelain := "worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n"

	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"git worktree list":             {out: porcelain},
		"tmux list-sessions":            {out: ""},
	}}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"list"})

	require.NoError(t, cmd.Execute())

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Len(t, lines, 1, "only header row expected")
}

func TestCreateCmd(t *testing.T) {
	isolateConfig(t)
	dir := newRepoDir(t)

	r := &fakeRunner{responses: map[string]response{
		"git rev-parse": {out: dir},
		"git fetch":     {},
		"git show-ref":  {err: fmt.Errorf("not found")},
		"git worktree":  {},
	}}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"create", "my-feature", "--no-tmux"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "Created worktree my-feature")
}

func TestCreateCmd_Error(t *testing.T) {
	isolateConfig(t)
	dir := newRepoDir(t)

	r := &fakeRunner{responses: map[string]response{
		"git rev-parse":    {out: dir},
		"git fetch":        {},
		"git show-ref":     {err: fmt.Errorf("not found")},
		"git worktree add": {err: fmt.Errorf("fatal: could not create")},
	}}

	cmd := cli.NewRootCmd("test", r)
	cmd.SetArgs([]string{"create", "bad-wt", "--no-tmux"})

	assert.Error(t, cmd.Execute())
}

func TestDeleteCmd_KeepBranch(t *testing.T) {
	isolateConfig(t)
	dir := newRepoDir(t)

	r := &fakeRunner{responses: map[string]response{
		"git rev-parse":    {out: dir},
		"tmux has-session": {err: fmt.Errorf("no session")},
		"git worktree":     {},
	}}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"delete", "my-feature", "--keep-branch"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "branch kept")
	assert.False(t, r.hasCall("branch -D"))
}

func TestDeleteCmd_DeleteBranch(t *testing.T) {
	isolateConfig(t)
	dir := newRepoDir(t)

	r := &fakeRunner{responses: map[string]response{
		"git rev-parse":    {out: dir},
		"tmux has-session": {err: fmt.Errorf("no session")},
		"git worktree":     {},
		"git branch":       {},
	}}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"delete", "my-feature", "-D"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "branch deleted")
	assert.True(t, r.hasCall("branch -D"))
}
