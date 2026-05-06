//go:build integration

package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	osexec "os/exec"
	"path/filepath"
	"testing"

	"github.com/davidmks/sarj/internal/cli"
	"github.com/davidmks/sarj/internal/exec"
	"github.com/davidmks/sarj/internal/tmux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireTmux(t *testing.T) {
	t.Helper()
	if _, err := osexec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
}

func initTestRepo(t *testing.T) string {
	t.Helper()
	// macOS resolves /var → /private/var; use EvalSymlinks so paths match
	// what git returns in worktree list.
	repoPath, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	r := &exec.DefaultRunner{Dir: repoPath}

	for _, cmd := range [][]string{
		{"git", "init"},
		{"git", "checkout", "-b", "main"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	} {
		_, err := r.Run(t.Context(), cmd[0], cmd[1:]...)
		require.NoError(t, err)
	}

	require.NoError(t, os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("# test"), 0o600))
	_, err = r.Run(t.Context(), "git", "add", ".")
	require.NoError(t, err)
	_, err = r.Run(t.Context(), "git", "commit", "-m", "init")
	require.NoError(t, err)

	return repoPath
}

func TestIntegration_DeleteFromInsideWorktree(t *testing.T) {
	requireTmux(t)
	isolateConfig(t)

	repoPath := initTestRepo(t)
	wtBase, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	r := &exec.DefaultRunner{Dir: repoPath}

	localCfg := "worktree_base = " + `"` + wtBase + `"` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(repoPath, ".sarj.local.toml"), []byte(localCfg), 0o600))

	cmd := cli.NewRootCmd("test", r)
	cmd.SetArgs([]string{"create", "test-wt", "--no-attach"})
	require.NoError(t, cmd.Execute())

	wtPath := filepath.Join(wtBase, "test-wt")
	assert.DirExists(t, wtPath)
	assert.True(t, tmux.SessionExists(t.Context(), r, "test-wt"))

	t.Cleanup(func() {
		// context.Background(), not t.Context(): t.Context is canceled
		// before Cleanup runs, which would block the kill-session subprocess.
		tmux.KillSession(context.Background(), r, "test-wt")
	})

	// Simulate running from inside the target worktree — the bug scenario
	// where CWD is deleted out from under the process.
	saveCwd(t)
	require.NoError(t, os.Chdir(wtPath))

	cmd = cli.NewRootCmd("test", &exec.DefaultRunner{})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"delete", "test-wt", "-D"})

	require.NoError(t, cmd.Execute())

	assert.Contains(t, buf.String(), "branch deleted")
	assert.NoDirExists(t, wtPath)
	assert.False(t, tmux.SessionExists(t.Context(), r, "test-wt"))

	rMain := &exec.DefaultRunner{Dir: repoPath}
	_, err = rMain.Run(t.Context(), "git", "show-ref", "--verify", "--quiet", "refs/heads/test-wt")
	assert.Error(t, err, "branch should be deleted")
}

// TestIntegration_ListEnriched exercises the full list pipeline against real
// git output: dirty detection, head info parsing, upstream resolution, and
// ahead/behind counting. Catches drift in any of those parsers.
func TestIntegration_ListEnriched(t *testing.T) {
	isolateConfig(t)

	// Bare repo serves as origin so the worktree branch can have an upstream.
	remotePath, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	rRemote := &exec.DefaultRunner{Dir: remotePath}
	_, err = rRemote.Run(t.Context(), "git", "init", "--bare", "--initial-branch=main")
	require.NoError(t, err)

	repoPath := initTestRepo(t)
	r := &exec.DefaultRunner{Dir: repoPath}
	_, err = r.Run(t.Context(), "git", "remote", "add", "origin", remotePath)
	require.NoError(t, err)
	_, err = r.Run(t.Context(), "git", "push", "-u", "origin", "main")
	require.NoError(t, err)

	wtBase, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	wtPath := filepath.Join(wtBase, "feat")
	_, err = r.Run(t.Context(), "git", "worktree", "add", "-b", "feat", wtPath)
	require.NoError(t, err)

	rWt := &exec.DefaultRunner{Dir: wtPath}
	_, err = rWt.Run(t.Context(), "git", "push", "-u", "origin", "feat")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(wtPath, "work.txt"), []byte("local work"), 0o600))
	_, err = rWt.Run(t.Context(), "git", "add", ".")
	require.NoError(t, err)
	_, err = rWt.Run(t.Context(), "git", "commit", "-m", "local commit")
	require.NoError(t, err)

	// Uncommitted file → dirty=true.
	require.NoError(t, os.WriteFile(filepath.Join(wtPath, "uncommitted.txt"), []byte("dirty"), 0o600))

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"list", "-o", "json"})
	require.NoError(t, cmd.Execute())

	var entries []map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entries))
	require.Len(t, entries, 1, "main filtered out, feat remains")

	e := entries[0]
	assert.Equal(t, "feat", e["name"])
	assert.Equal(t, "feat", e["branch"])
	assert.Equal(t, true, e["dirty"], "uncommitted file should mark dirty")
	assert.Nil(t, e["status"], "status null when no [status] hook is configured")

	head := e["head"].(map[string]any)
	assert.Equal(t, "local commit", head["subject"])
	assert.NotEmpty(t, head["sha"])
	assert.NotEmpty(t, head["date"])

	up := e["upstream"].(map[string]any)
	assert.Equal(t, "origin", up["remote"])
	assert.Equal(t, "feat", up["branch"])
	assert.Equal(t, float64(1), up["ahead"], "1 local commit ahead of origin")
	assert.Equal(t, float64(0), up["behind"])
}

// TestIntegration_ListWithStatusHook exercises the full chain: a real
// .sarj.toml with [status] command, real git worktrees, real shell exec
// of the templated hook, and real list rendering. Catches regressions at
// the seams between config, status, and list that fakeRunner can't see.
func TestIntegration_ListWithStatusHook(t *testing.T) {
	isolateConfig(t)

	repoPath := initTestRepo(t)
	r := &exec.DefaultRunner{Dir: repoPath}

	wtBase, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	wtPath := filepath.Join(wtBase, "feat")
	_, err = r.Run(t.Context(), "git", "worktree", "add", "-b", "feat", wtPath)
	require.NoError(t, err)

	// Hook echoes $BRANCH so we can verify env-var injection end-to-end.
	cfg := "[status]\ncommand = \"echo merged-$BRANCH\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(repoPath, ".sarj.toml"), []byte(cfg), 0o600))

	cmd := cli.NewRootCmd("test", r)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"list", "-o", "json"})
	require.NoError(t, cmd.Execute())

	var entries []map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entries))
	require.Len(t, entries, 1)
	assert.Equal(t, "merged-feat", entries[0]["status"])

	// Text output should now show the STATUS column.
	buf.Reset()
	cmd2 := cli.NewRootCmd("test", r)
	cmd2.SetOut(buf)
	cmd2.SetErr(new(bytes.Buffer))
	cmd2.SetArgs([]string{"list"})
	require.NoError(t, cmd2.Execute())
	assert.Contains(t, buf.String(), "STATUS")
	assert.Contains(t, buf.String(), "merged-feat")
}
