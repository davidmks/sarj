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

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain":                          {out: porcelain},
		"git fetch":                                              {},
		"git show-ref --verify --quiet refs/heads/my-feature":    {err: fmt.Errorf("not found")},
		"git show-ref --verify --quiet refs/remotes/origin/main": {},
		"git worktree": {},
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

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain":                          {out: porcelain},
		"git fetch":                                              {},
		"git show-ref --verify --quiet refs/heads/bad-wt":        {err: fmt.Errorf("not found")},
		"git show-ref --verify --quiet refs/remotes/origin/main": {},
		"git worktree add": {err: fmt.Errorf("fatal: could not create")},
	}}

	cmd := cli.NewRootCmd("test", r)
	cmd.SetArgs([]string{"create", "bad-wt", "--no-tmux"})

	assert.Error(t, cmd.Execute())
}

func TestDeleteCmd_KeepBranch(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)
	wtPath := filepath.Join(dir, "wt", "my-feature")

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + wtPath + "\nHEAD def\nbranch refs/heads/my-feature\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"tmux has-session":              {err: fmt.Errorf("no session")},
		"git worktree":                  {},
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
	saveCwd(t)
	dir := newRepoDir(t)
	wtPath := filepath.Join(dir, "wt", "my-feature")

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + wtPath + "\nHEAD def\nbranch refs/heads/my-feature\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"tmux has-session":              {err: fmt.Errorf("no session")},
		"git worktree":                  {},
		"git branch":                    {},
	}}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"delete", "my-feature", "-D"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "branch deleted")
	assert.True(t, r.hasCall("branch -D my-feature"))
}

func TestDeleteCmd_DeleteBranch_DivergentName(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)
	wtPath := filepath.Join(dir, "wt", "issue-1")

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + wtPath + "\nHEAD def\nbranch refs/heads/fix/1-delete-bug\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"tmux has-session":              {err: fmt.Errorf("no session")},
		"git worktree":                  {},
		"git branch":                    {},
	}}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"delete", "issue-1", "-D"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "branch deleted")
	assert.True(t, r.hasCall("branch -D fix/1-delete-bug"))
	assert.False(t, r.hasCall("branch -D issue-1"))
}

func TestDeleteCmd_CleanupBeforeKill(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)
	wtPath := filepath.Join(dir, "wt", "my-feature")

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + wtPath + "\nHEAD def\nbranch refs/heads/my-feature\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"tmux has-session":              {},
		"git worktree":                  {},
		"git branch":                    {},
	}}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"delete", "my-feature", "-D"})

	require.NoError(t, cmd.Execute())

	wtRemove := r.indexOfCall("worktree remove")
	branchDelete := r.indexOfCall("branch -D")
	sessionKill := r.indexOfCall("kill-session")

	assert.Greater(t, wtRemove, -1, "worktree remove should be called")
	assert.Greater(t, branchDelete, -1, "branch -D should be called")
	assert.Greater(t, sessionKill, -1, "kill-session should be called")
	assert.Less(t, wtRemove, sessionKill, "worktree remove must happen before kill-session")
	assert.Less(t, branchDelete, sessionKill, "branch delete must happen before kill-session")
}

func TestDeleteCmd_SwitchesAwayBeforeKill(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")
	wtPath := filepath.Join(dir, "wt", "my-feature")

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + wtPath + "\nHEAD def\nbranch refs/heads/my-feature\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain":           {out: porcelain},
		"tmux has-session":                        {},
		"tmux display-message -p #{session_name}": {out: "my-feature"},
		"tmux switch-client -l":                   {},
		"git worktree":                            {},
	}}

	cmd := cli.NewRootCmd("test", r)
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{"delete", "my-feature", "--keep-branch"})

	require.NoError(t, cmd.Execute())

	switchClient := r.indexOfCall("switch-client -l")
	sessionKill := r.indexOfCall("kill-session")

	assert.Greater(t, switchClient, -1, "switch-client should be called")
	assert.Less(t, switchClient, sessionKill, "switch must happen before kill")
}

func TestDeleteCmd_NoSwitchWhenOutsideTmux(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)
	t.Setenv("TMUX", "")
	wtPath := filepath.Join(dir, "wt", "my-feature")

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + wtPath + "\nHEAD def\nbranch refs/heads/my-feature\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"tmux has-session":              {err: fmt.Errorf("no session")},
		"git worktree":                  {},
	}}

	cmd := cli.NewRootCmd("test", r)
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{"delete", "my-feature", "--keep-branch"})

	require.NoError(t, cmd.Execute())
	assert.False(t, r.hasCall("switch-client"))
}

func TestDeleteCmd_NoSwitchWhenDifferentSession(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")
	wtPath := filepath.Join(dir, "wt", "my-feature")

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + wtPath + "\nHEAD def\nbranch refs/heads/my-feature\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain":           {out: porcelain},
		"tmux has-session":                        {err: fmt.Errorf("no session")},
		"tmux display-message -p #{session_name}": {out: "other-session"},
		"git worktree":                            {},
	}}

	cmd := cli.NewRootCmd("test", r)
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{"delete", "my-feature", "--keep-branch"})

	require.NoError(t, cmd.Execute())
	assert.False(t, r.hasCall("switch-client"))
}
