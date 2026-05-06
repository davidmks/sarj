package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidmks/sarj/internal/cli"
	"github.com/davidmks/sarj/internal/tmux"
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

func TestListCmd_SlashBranch(t *testing.T) {
	isolateConfig(t)
	branch := "feat/auth"
	porcelain := "worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree /wt/feat-auth\nHEAD def\nbranch refs/heads/" + branch + "\n\n"

	sessionName := tmux.SanitizeName(branch)
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"tmux list-sessions":            {out: sessionName},
	}}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"list"})

	require.NoError(t, cmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "feat-auth")
	assert.Contains(t, out, "active")
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

func TestListCmd_NewColumns(t *testing.T) {
	isolateConfig(t)
	porcelain := "worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree /wt/feat\nHEAD def\nbranch refs/heads/feat\n\n"

	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain":                 {out: porcelain},
		"tmux list-sessions":                            {out: ""},
		"git -C /wt/feat status --porcelain":            {out: " M file.go\n"},
		"git -C /wt/feat log":                           {out: "2026-05-04T10:23:00Z\nfix things\n"},
		"git -C /wt/feat rev-parse":                     {out: "origin/feat\n"},
		"git -C /wt/feat rev-list --left-right --count": {out: "3\t1\n"},
	}}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"list"})
	require.NoError(t, cmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "AHEAD/BEHIND")
	assert.Contains(t, out, "AGE")
	assert.Contains(t, out, "DIRTY")
	assert.Contains(t, out, "+3/-1")
	assert.Contains(t, out, "*")
}

func TestListCmd_NoUpstream(t *testing.T) {
	isolateConfig(t)
	porcelain := "worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree /wt/scratch\nHEAD def\nbranch refs/heads/scratch\n\n"

	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"tmux list-sessions":            {out: ""},
		"git -C /wt/scratch rev-parse":  {err: fmt.Errorf("no upstream")},
	}}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"list"})
	require.NoError(t, cmd.Execute())

	assert.Contains(t, buf.String(), "-/-")
}

func TestListCmd_JSON(t *testing.T) {
	isolateConfig(t)
	porcelain := "worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree /wt/feat\nHEAD def123\nbranch refs/heads/feat\n\n"

	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain":                 {out: porcelain},
		"tmux list-sessions":                            {out: "feat"},
		"git -C /wt/feat status --porcelain":            {out: ""},
		"git -C /wt/feat log":                           {out: "2026-05-04T10:23:00Z\nfix things\n"},
		"git -C /wt/feat rev-parse":                     {out: "origin/feat\n"},
		"git -C /wt/feat rev-list --left-right --count": {out: "0\t0\n"},
	}}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"list", "-o", "json"})
	require.NoError(t, cmd.Execute())

	var entries []map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entries))
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "feat", e["name"])
	assert.Equal(t, "/wt/feat", e["path"])
	assert.Equal(t, "feat", e["branch"])
	assert.Equal(t, false, e["dirty"])
	assert.Nil(t, e["status"], "status null in PR 1")

	head := e["head"].(map[string]any)
	assert.Equal(t, "def123", head["sha"])
	assert.Equal(t, "fix things", head["subject"])
	assert.Equal(t, "2026-05-04T10:23:00Z", head["date"])

	up := e["upstream"].(map[string]any)
	assert.Equal(t, "origin", up["remote"])
	assert.Equal(t, "feat", up["branch"])

	tmuxObj := e["tmux"].(map[string]any)
	assert.Equal(t, "feat", tmuxObj["session"])
	assert.Equal(t, true, tmuxObj["active"])
}

func TestListCmd_JSON_NullableFields(t *testing.T) {
	isolateConfig(t)
	porcelain := "worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree /wt/foo\nHEAD def\nbranch refs/heads/foo\n\n"

	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"tmux list-sessions":            {err: fmt.Errorf("tmux not running")},
		"git -C /wt/foo rev-parse":      {err: fmt.Errorf("no upstream")},
	}}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"list", "-o", "json"})
	require.NoError(t, cmd.Execute())

	var entries []map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entries))
	require.Len(t, entries, 1)
	assert.Nil(t, entries[0]["upstream"])
	assert.Nil(t, entries[0]["tmux"])
	assert.Nil(t, entries[0]["status"])
}

func TestListCmd_StatusColumn(t *testing.T) {
	isolateConfig(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".sarj.toml"),
		[]byte("[status]\ncommand = \"echo merged\"\n"), 0o600))

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree /wt/feat\nHEAD def\nbranch refs/heads/feat\n\n"

	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain":                 {out: porcelain},
		"tmux list-sessions":                            {out: ""},
		"git -C /wt/feat status --porcelain":            {out: ""},
		"git -C /wt/feat log":                           {out: "2026-05-04T10:23:00Z\nfix things\n"},
		"git -C /wt/feat rev-parse":                     {out: "origin/feat\n"},
		"git -C /wt/feat rev-list --left-right --count": {out: "0\t0\n"},
		"sh -c": {out: "merged"},
	}}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"list"})
	require.NoError(t, cmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "STATUS")
	assert.Contains(t, out, "merged")
}

func TestListCmd_StatusOmittedWithoutHook(t *testing.T) {
	isolateConfig(t)
	porcelain := "worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree /wt/feat\nHEAD def\nbranch refs/heads/feat\n\n"

	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"tmux list-sessions":            {out: ""},
	}}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"list"})
	require.NoError(t, cmd.Execute())

	assert.NotContains(t, buf.String(), "STATUS")
}

func TestListCmd_JSON_StatusPopulated(t *testing.T) {
	isolateConfig(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".sarj.toml"),
		[]byte("[status]\ncommand = \"echo merged\"\n"), 0o600))

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree /wt/feat\nHEAD def123\nbranch refs/heads/feat\n\n"

	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain":                 {out: porcelain},
		"tmux list-sessions":                            {out: ""},
		"git -C /wt/feat status --porcelain":            {out: ""},
		"git -C /wt/feat log":                           {out: "2026-05-04T10:23:00Z\nfix things\n"},
		"git -C /wt/feat rev-parse":                     {out: "origin/feat\n"},
		"git -C /wt/feat rev-list --left-right --count": {out: "0\t0\n"},
		"sh -c": {out: "merged"},
	}}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"list", "-o", "json"})
	require.NoError(t, cmd.Execute())

	var entries []map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entries))
	require.Len(t, entries, 1)
	assert.Equal(t, "merged", entries[0]["status"])
}

func TestListCmd_JSON_StatusUnknownOnFailure(t *testing.T) {
	isolateConfig(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".sarj.toml"),
		[]byte("[status]\ncommand = \"some-cmd\"\n"), 0o600))

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree /wt/feat\nHEAD def\nbranch refs/heads/feat\n\n"

	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"tmux list-sessions":            {out: ""},
		"git -C /wt/feat rev-parse":     {err: fmt.Errorf("no upstream")},
		"sh -c":                         {err: fmt.Errorf("hook failed")},
	}}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"list", "-o", "json"})
	require.NoError(t, cmd.Execute())

	var entries []map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entries))
	require.Len(t, entries, 1)
	assert.Equal(t, "unknown", entries[0]["status"])
}

func TestListCmd_InvalidOutputFlag(t *testing.T) {
	isolateConfig(t)
	r := &fakeRunner{}
	cmd := cli.NewRootCmd("test", r)
	cmd.SetArgs([]string{"list", "-o", "xml"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid -o value")
}

func TestCreateCmd(t *testing.T) {
	isolateConfig(t)
	dir := newRepoDir(t)

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"git fetch":                     {},
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
		"git worktree list --porcelain": {out: porcelain},
		"git fetch":                     {},
		"git show-ref --verify --quiet refs/heads/bad-wt":        {err: fmt.Errorf("not found")},
		"git show-ref --verify --quiet refs/remotes/origin/main": {},
		"git worktree add": {err: fmt.Errorf("fatal: could not create")},
	}}

	cmd := cli.NewRootCmd("test", r)
	cmd.SetArgs([]string{"create", "bad-wt", "--no-tmux"})

	assert.Error(t, cmd.Execute())
}

func TestCreateCmd_AutoSetupFalseSkipsSetup(t *testing.T) {
	isolateConfig(t)
	dir := newRepoDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".sarj.toml"), []byte(`
default_branch = "main"
setup_command = "echo running setup"
auto_setup = false
`), 0o600))

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"git fetch":                     {},
		"git show-ref --verify --quiet refs/heads/my-feature":    {err: fmt.Errorf("not found")},
		"git show-ref --verify --quiet refs/remotes/origin/main": {},
		"git worktree": {},
	}}

	cmd := cli.NewRootCmd("test", r)
	cmd.SetArgs([]string{"create", "my-feature", "--no-tmux"})

	require.NoError(t, cmd.Execute())
	assert.False(t, r.hasCall("echo running setup"), "auto_setup = false should skip the setup command")
}

func TestCreateCmd_NoSetupFalseOverridesAutoSetupFalse(t *testing.T) {
	isolateConfig(t)
	dir := newRepoDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".sarj.toml"), []byte(`
default_branch = "main"
setup_command = "echo running setup"
auto_setup = false
`), 0o600))

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"git fetch":                     {},
		"git show-ref --verify --quiet refs/heads/my-feature":    {err: fmt.Errorf("not found")},
		"git show-ref --verify --quiet refs/remotes/origin/main": {},
		"git worktree": {},
	}}

	cmd := cli.NewRootCmd("test", r)
	cmd.SetArgs([]string{"create", "my-feature", "--no-tmux", "--no-setup=false"})

	require.NoError(t, cmd.Execute())
	assert.True(t, r.hasCall("echo running setup"), "explicit --no-setup=false should run setup despite auto_setup = false")
}

func TestCreateCmd_NoSetupClearsPlaceholder(t *testing.T) {
	isolateConfig(t)
	dir := newRepoDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".sarj.toml"), []byte(`
default_branch = "main"
setup_command = "make setup"

[[tmux.windows]]
name = "dev"
command = "{{.SetupCommand}}"
`), 0o600))

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"git fetch":                     {},
		"git show-ref --verify --quiet refs/heads/my-feature":    {err: fmt.Errorf("not found")},
		"git show-ref --verify --quiet refs/remotes/origin/main": {},
		"git worktree": {},
		"tmux":         {},
	}}

	cmd := cli.NewRootCmd("test", r)
	cmd.SetArgs([]string{"create", "my-feature", "--no-setup", "--no-attach"})

	require.NoError(t, cmd.Execute())
	assert.False(t, r.hasCall("make setup"), "--no-setup should clear {{.SetupCommand}} so the pane does not run setup")
}

func TestCreateCmd_PlaceholderSubstitutedByDefault(t *testing.T) {
	isolateConfig(t)
	dir := newRepoDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".sarj.toml"), []byte(`
default_branch = "main"
setup_command = "make setup"

[[tmux.windows]]
name = "dev"
command = "{{.SetupCommand}}"
`), 0o600))

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"git fetch":                     {},
		"git show-ref --verify --quiet refs/heads/my-feature":    {err: fmt.Errorf("not found")},
		"git show-ref --verify --quiet refs/remotes/origin/main": {},
		"git worktree": {},
		"tmux":         {},
	}}

	cmd := cli.NewRootCmd("test", r)
	cmd.SetArgs([]string{"create", "my-feature", "--no-attach"})

	require.NoError(t, cmd.Execute())
	assert.True(t, r.hasCall("send-keys -t my-feature:dev clear && make setup Enter"), "without --no-setup, placeholder should resolve to setup_command")
}

func TestDeleteCmd_KeepBranch(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)
	wtPath := filepath.Join(dir, "wt", "my-feature")
	fakeWorktreeDir(t, wtPath)

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
	fakeWorktreeDir(t, wtPath)

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
	fakeWorktreeDir(t, wtPath)

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

func TestDeleteCmd_NotFound(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
	}}

	cmd := cli.NewRootCmd("test", r)
	cmd.SetArgs([]string{"delete", "ghost", "--keep-branch"})

	err := cmd.Execute()
	assert.ErrorContains(t, err, "worktree not found: ghost")
}

func TestDeleteCmd_StaleEntry(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)
	wtPath := filepath.Join(dir, "wt", "stale-wt")
	// Intentionally NOT creating the directory — simulates a stale entry.

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + wtPath + "\nHEAD def\nbranch refs/heads/stale-wt\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"tmux has-session":              {err: fmt.Errorf("no session")},
		"git worktree":                  {},
		"git branch":                    {},
	}}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"delete", "stale-wt", "-D"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "branch deleted")
	assert.False(t, r.hasCall("worktree remove"), "should skip remove for missing directory")
	assert.True(t, r.hasCall("worktree prune"))
}

func TestDeleteCmd_StaleEntry_DivergentName(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)
	wtPath := filepath.Join(dir, "wt", "issue-3")
	// Intentionally NOT creating the directory.

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + wtPath + "\nHEAD def\nbranch refs/heads/fix/3-combo-bug\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"tmux has-session":              {err: fmt.Errorf("no session")},
		"git worktree":                  {},
		"git branch":                    {},
	}}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"delete", "issue-3", "-D"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "branch deleted")
	assert.True(t, r.hasCall("branch -D fix/3-combo-bug"))
	assert.False(t, r.hasCall("branch -D issue-3"))
	assert.True(t, r.hasCall("worktree prune"))
}

func TestDeleteCmd_CleanupBeforeKill(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)
	wtPath := filepath.Join(dir, "wt", "my-feature")
	fakeWorktreeDir(t, wtPath)

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
	fakeWorktreeDir(t, wtPath)

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
	fakeWorktreeDir(t, wtPath)

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

func TestDeleteCmd_InferFromCwd(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir, err := filepath.EvalSymlinks(newRepoDir(t))
	require.NoError(t, err)
	wtBase, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	wtPath := filepath.Join(wtBase, "my-feature")
	fakeWorktreeDir(t, wtPath)

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + wtPath + "\nHEAD def\nbranch refs/heads/my-feature\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"tmux has-session":              {err: fmt.Errorf("no session")},
		"git worktree":                  {},
	}}

	require.NoError(t, os.Chdir(wtPath))

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetIn(strings.NewReader("y\n"))
	cmd.SetArgs([]string{"delete", "--keep-branch"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "my-feature")
	assert.True(t, r.hasCall("worktree remove"))
}

func TestDeleteCmd_InferFromCwd_Decline(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir, err := filepath.EvalSymlinks(newRepoDir(t))
	require.NoError(t, err)
	wtBase, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	wtPath := filepath.Join(wtBase, "my-feature")
	fakeWorktreeDir(t, wtPath)

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + wtPath + "\nHEAD def\nbranch refs/heads/my-feature\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
	}}

	require.NoError(t, os.Chdir(wtPath))

	cmd := cli.NewRootCmd("test", r)
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetIn(strings.NewReader("n\n"))
	cmd.SetArgs([]string{"delete", "--keep-branch"})

	require.NoError(t, cmd.Execute())
	assert.False(t, r.hasCall("worktree remove"))
}

func TestDeleteCmd_InferFromCwd_Subdirectory(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir, err := filepath.EvalSymlinks(newRepoDir(t))
	require.NoError(t, err)
	wtBase, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	wtPath := filepath.Join(wtBase, "my-feature")
	subdir := filepath.Join(wtPath, "src", "pkg")
	fakeWorktreeDir(t, subdir)

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + wtPath + "\nHEAD def\nbranch refs/heads/my-feature\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"tmux has-session":              {err: fmt.Errorf("no session")},
		"git worktree":                  {},
	}}

	require.NoError(t, os.Chdir(subdir))

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetIn(strings.NewReader("y\n"))
	cmd.SetArgs([]string{"delete", "--keep-branch"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "my-feature")
	assert.True(t, r.hasCall("worktree remove"))
}

func TestDeleteCmd_InferFromCwd_MainWorktree(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir, err := filepath.EvalSymlinks(newRepoDir(t))
	require.NoError(t, err)

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
	}}

	require.NoError(t, os.Chdir(dir))

	cmd := cli.NewRootCmd("test", r)
	cmd.SetArgs([]string{"delete", "--keep-branch"})

	err = cmd.Execute()
	assert.ErrorContains(t, err, "cannot delete the main worktree")
}

func TestDeleteCmd_InferFromCwd_NotInWorktree(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)
	outside := t.TempDir()

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
	}}

	require.NoError(t, os.Chdir(outside))

	cmd := cli.NewRootCmd("test", r)
	cmd.SetArgs([]string{"delete", "--keep-branch"})

	err := cmd.Execute()
	assert.ErrorContains(t, err, "not inside a worktree")
}

func TestDeleteCmd_MultiArg(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)
	wtA := filepath.Join(dir, "wt", "feat-a")
	wtB := filepath.Join(dir, "wt", "feat-b")
	fakeWorktreeDir(t, wtA)
	fakeWorktreeDir(t, wtB)

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + wtA + "\nHEAD def\nbranch refs/heads/feat-a\n\n" +
		"worktree " + wtB + "\nHEAD ghi\nbranch refs/heads/feat-b\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"tmux has-session":              {err: fmt.Errorf("no session")},
		"git worktree":                  {},
	}}

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"delete", "feat-a", "feat-b", "-y"})

	require.NoError(t, cmd.Execute())
	out := buf.String()
	assert.Contains(t, out, "Deleted worktree feat-a")
	assert.Contains(t, out, "Deleted worktree feat-b")
	assert.False(t, r.hasCall("branch -D"), "-y defaults to keep-branch")
}

func TestDeleteCmd_MultiArg_UnknownNameAborts(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)
	wtA := filepath.Join(dir, "wt", "feat-a")
	fakeWorktreeDir(t, wtA)

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + wtA + "\nHEAD def\nbranch refs/heads/feat-a\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
	}}

	cmd := cli.NewRootCmd("test", r)
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{"delete", "feat-a", "ghost", "-y"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghost")
	assert.False(t, r.hasCall("worktree remove"), "no side effect on unknown name")
}

func TestDeleteCmd_MultiArg_PartialFailure(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)
	wtA := filepath.Join(dir, "wt", "feat-a")
	wtB := filepath.Join(dir, "wt", "feat-b")
	fakeWorktreeDir(t, wtA)
	fakeWorktreeDir(t, wtB)

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + wtA + "\nHEAD def\nbranch refs/heads/feat-a\n\n" +
		"worktree " + wtB + "\nHEAD ghi\nbranch refs/heads/feat-b\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain":      {out: porcelain},
		"tmux has-session":                   {err: fmt.Errorf("no session")},
		"git worktree remove --force " + wtA: {err: fmt.Errorf("locked worktree")},
		"git worktree":                       {},
	}}

	cmd := cli.NewRootCmd("test", r)
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{"delete", "feat-a", "feat-b", "-y"})

	err := cmd.Execute()
	require.Error(t, err, "partial failure must return non-zero")
	assert.Contains(t, err.Error(), "feat-a")
	assert.True(t, r.hasCall("worktree remove --force "+wtB), "feat-b still attempted after feat-a failed")
}

func TestDeleteCmd_YesFlag_DeleteBranch(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)
	wtPath := filepath.Join(dir, "wt", "my-feature")
	fakeWorktreeDir(t, wtPath)

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + wtPath + "\nHEAD def\nbranch refs/heads/my-feature\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"tmux has-session":              {err: fmt.Errorf("no session")},
		"git worktree":                  {},
		"git branch":                    {},
	}}

	cmd := cli.NewRootCmd("test", r)
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{"delete", "my-feature", "-y", "-D"})

	require.NoError(t, cmd.Execute())
	assert.True(t, r.hasCall("branch -D my-feature"), "-y -D should delete the branch")
}

func TestDeleteCmd_NoSwitchWhenDifferentSession(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")
	wtPath := filepath.Join(dir, "wt", "my-feature")
	fakeWorktreeDir(t, wtPath)

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

// writeStatusConfig appends a [status] section whose command is a no-op
// `check $BRANCH` — the fakeRunner keys responses on the BRANCH env suffix,
// so the literal command body is irrelevant.
func writeStatusConfig(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, ".sarj.toml")
	existing, err := os.ReadFile(path)
	require.NoError(t, err)
	added := "\n[status]\ncommand = \"check $BRANCH\"\n"
	require.NoError(t, os.WriteFile(path, append(existing, []byte(added)...), 0o600)) //nolint:gosec // test helper, path is t.TempDir()
}

func TestDeleteCmd_StateRequiresHook(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
	}}

	cmd := cli.NewRootCmd("test", r)
	cmd.SetArgs([]string{"delete", "--state", "merged", "-y"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--state")
	assert.Contains(t, err.Error(), "[status]")
}

func TestDeleteCmd_StateFiltersTargets(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)
	writeStatusConfig(t, dir)

	wtA := filepath.Join(dir, "wt", "a")
	wtB := filepath.Join(dir, "wt", "b")
	wtC := filepath.Join(dir, "wt", "c")
	fakeWorktreeDir(t, wtA)
	fakeWorktreeDir(t, wtB)
	fakeWorktreeDir(t, wtC)

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + wtA + "\nHEAD a1\nbranch refs/heads/a\n\n" +
		"worktree " + wtB + "\nHEAD b1\nbranch refs/heads/b\n\n" +
		"worktree " + wtC + "\nHEAD c1\nbranch refs/heads/c\n\n"

	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"tmux has-session":              {err: fmt.Errorf("no session")},
		"git worktree":                  {},
		"sh -c check $BRANCH BRANCH=a":  {out: "merged"},
		"sh -c check $BRANCH BRANCH=b":  {out: "merged"},
		"sh -c check $BRANCH BRANCH=c":  {out: "open"},
	}}

	cmd := cli.NewRootCmd("test", r)
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"delete", "--state", "merged", "-y"})

	require.NoError(t, cmd.Execute())
	assert.True(t, r.hasCall("worktree remove --force "+wtA), "a should be deleted")
	assert.True(t, r.hasCall("worktree remove --force "+wtB), "b should be deleted")
	assert.False(t, r.hasCall("worktree remove --force "+wtC), "c (open) should not be deleted")
}

func TestDeleteCmd_StateMultipleValues(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)
	writeStatusConfig(t, dir)

	wtA := filepath.Join(dir, "wt", "a")
	wtB := filepath.Join(dir, "wt", "b")
	wtC := filepath.Join(dir, "wt", "c")
	fakeWorktreeDir(t, wtA)
	fakeWorktreeDir(t, wtB)
	fakeWorktreeDir(t, wtC)

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + wtA + "\nHEAD a1\nbranch refs/heads/a\n\n" +
		"worktree " + wtB + "\nHEAD b1\nbranch refs/heads/b\n\n" +
		"worktree " + wtC + "\nHEAD c1\nbranch refs/heads/c\n\n"

	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"tmux has-session":              {err: fmt.Errorf("no session")},
		"git worktree":                  {},
		"sh -c check $BRANCH BRANCH=a":  {out: "merged"},
		"sh -c check $BRANCH BRANCH=b":  {out: "closed"},
		"sh -c check $BRANCH BRANCH=c":  {out: "open"},
	}}

	cmd := cli.NewRootCmd("test", r)
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"delete", "--state", "merged,closed", "-y"})

	require.NoError(t, cmd.Execute())
	assert.True(t, r.hasCall("worktree remove --force "+wtA))
	assert.True(t, r.hasCall("worktree remove --force "+wtB))
	assert.False(t, r.hasCall("worktree remove --force "+wtC))
}

func TestDeleteCmd_StateCaseInsensitive(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)
	writeStatusConfig(t, dir)

	wtA := filepath.Join(dir, "wt", "a")
	wtB := filepath.Join(dir, "wt", "b")
	fakeWorktreeDir(t, wtA)
	fakeWorktreeDir(t, wtB)

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + wtA + "\nHEAD a1\nbranch refs/heads/a\n\n" +
		"worktree " + wtB + "\nHEAD b1\nbranch refs/heads/b\n\n"

	// Hook returns UPPERCASE (gh-style); user types lowercase.
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"tmux has-session":              {err: fmt.Errorf("no session")},
		"git worktree":                  {},
		"sh -c check $BRANCH BRANCH=a":  {out: "MERGED"},
		"sh -c check $BRANCH BRANCH=b":  {out: "OPEN"},
	}}

	cmd := cli.NewRootCmd("test", r)
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"delete", "--state", "merged", "-y"})

	require.NoError(t, cmd.Execute())
	assert.True(t, r.hasCall("worktree remove --force "+wtA), "MERGED matches --state merged")
	assert.False(t, r.hasCall("worktree remove --force "+wtB), "OPEN should not match")
}

func TestDeleteCmd_StateNoMatchPreservesWorktrees(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)
	writeStatusConfig(t, dir)

	wtA := filepath.Join(dir, "wt", "a")
	fakeWorktreeDir(t, wtA)

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + wtA + "\nHEAD a1\nbranch refs/heads/a\n\n"

	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"tmux has-session":              {err: fmt.Errorf("no session")},
		"sh -c check $BRANCH BRANCH=a":  {out: "OPEN"},
	}}

	cmd := cli.NewRootCmd("test", r)
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	// User-typed state doesn't match anything observed. Must not delete.
	cmd.SetArgs([]string{"delete", "--state", "nonexistent", "-y"})

	require.NoError(t, cmd.Execute())
	assert.False(t, r.hasCall("worktree remove"), "no worktrees should be removed")
}

func TestDeleteCmd_StateWithNamedArgs(t *testing.T) {
	isolateConfig(t)
	saveCwd(t)
	dir := newRepoDir(t)
	writeStatusConfig(t, dir)

	wtA := filepath.Join(dir, "wt", "a")
	wtB := filepath.Join(dir, "wt", "b")
	wtC := filepath.Join(dir, "wt", "c")
	fakeWorktreeDir(t, wtA)
	fakeWorktreeDir(t, wtB)
	fakeWorktreeDir(t, wtC)

	porcelain := "worktree " + dir + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + wtA + "\nHEAD a1\nbranch refs/heads/a\n\n" +
		"worktree " + wtB + "\nHEAD b1\nbranch refs/heads/b\n\n" +
		"worktree " + wtC + "\nHEAD c1\nbranch refs/heads/c\n\n"

	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {out: porcelain},
		"tmux has-session":              {err: fmt.Errorf("no session")},
		"git worktree":                  {},
		"sh -c check $BRANCH BRANCH=a":  {out: "merged"},
		"sh -c check $BRANCH BRANCH=b":  {out: "open"},
		"sh -c check $BRANCH BRANCH=c":  {out: "merged"},
	}}

	cmd := cli.NewRootCmd("test", r)
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	// Named args narrow the candidate pool: only a and b are considered,
	// then state filters to merged. c is not considered even though merged.
	cmd.SetArgs([]string{"delete", "a", "b", "--state", "merged", "-y"})

	require.NoError(t, cmd.Execute())
	assert.True(t, r.hasCall("worktree remove --force "+wtA))
	assert.False(t, r.hasCall("worktree remove --force "+wtB), "b is open, filtered out")
	assert.False(t, r.hasCall("worktree remove --force "+wtC), "c not in named args")
}
