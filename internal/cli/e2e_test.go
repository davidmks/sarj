//go:build integration

package cli_test

import (
	"bytes"
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
		_, err := r.Run(cmd[0], cmd[1:]...)
		require.NoError(t, err)
	}

	require.NoError(t, os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("# test"), 0o600))
	_, err = r.Run("git", "add", ".")
	require.NoError(t, err)
	_, err = r.Run("git", "commit", "-m", "init")
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
	assert.True(t, tmux.SessionExists(r, "test-wt"))

	t.Cleanup(func() {
		tmux.KillSession(r, "test-wt")
	})

	// Simulate running from inside the target worktree — the bug scenario
	// where CWD is deleted out from under the process.
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(wtPath))
	t.Cleanup(func() { os.Chdir(origDir) })

	cmd = cli.NewRootCmd("test", &exec.DefaultRunner{})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"delete", "test-wt", "-D"})

	require.NoError(t, cmd.Execute())

	assert.Contains(t, buf.String(), "branch deleted")
	assert.NoDirExists(t, wtPath)
	assert.False(t, tmux.SessionExists(r, "test-wt"))

	rMain := &exec.DefaultRunner{Dir: repoPath}
	_, err = rMain.Run("git", "show-ref", "--verify", "--quiet", "refs/heads/test-wt")
	assert.Error(t, err, "branch should be deleted")
}
