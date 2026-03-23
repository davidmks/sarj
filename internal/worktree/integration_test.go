//go:build integration

package worktree_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/davidmks/sarj/internal/config"
	"github.com/davidmks/sarj/internal/exec"
	"github.com/davidmks/sarj/internal/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initTestRepo creates a real git repo with an initial commit.
func initTestRepo(t *testing.T) (repoPath string, runner *exec.DefaultRunner) {
	t.Helper()

	repoPath = t.TempDir()
	runner = &exec.DefaultRunner{Dir: repoPath}

	for _, cmd := range [][]string{
		{"git", "init"},
		{"git", "checkout", "-b", "main"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	} {
		_, err := runner.Run(cmd[0], cmd[1:]...)
		require.NoError(t, err)
	}

	require.NoError(t, os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("# test"), 0o600))
	_, err := runner.Run("git", "add", ".")
	require.NoError(t, err)
	_, err = runner.Run("git", "commit", "-m", "init")
	require.NoError(t, err)

	return repoPath, runner
}

func TestIntegration_CreateListDelete(t *testing.T) {
	_, r := initTestRepo(t)
	wtBase := t.TempDir()

	cfg := &config.Config{
		WorktreeBase:  wtBase,
		DefaultBranch: "main",
	}

	// Create
	wt, err := worktree.Create(r, cfg, worktree.CreateOpts{
		Name:      "test-branch",
		SkipSetup: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "test-branch", wt.Branch)
	assert.DirExists(t, wt.Path)

	// List — should show main + test-branch
	wts, err := worktree.List(r)
	require.NoError(t, err)
	assert.Len(t, wts, 2)

	// Delete
	err = worktree.Delete(r, cfg, worktree.DeleteOpts{
		Name:         "test-branch",
		DeleteBranch: true,
	})
	require.NoError(t, err)
	assert.NoDirExists(t, wt.Path)

	// Only main remains
	wts, err = worktree.List(r)
	require.NoError(t, err)
	assert.Len(t, wts, 1)
}

func TestIntegration_CreateWithSymlinks(t *testing.T) {
	repoPath, r := initTestRepo(t)
	wtBase := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(repoPath, ".env"), []byte("SECRET=x"), 0o600))

	cfg := &config.Config{
		WorktreeBase:  wtBase,
		DefaultBranch: "main",
		Symlinks:      []string{".env"},
	}

	wt, err := worktree.Create(r, cfg, worktree.CreateOpts{
		Name:      "symlink-test",
		SkipSetup: true,
	})
	require.NoError(t, err)

	target, err := os.Readlink(filepath.Join(wt.Path, ".env"))
	require.NoError(t, err)
	// macOS resolves /var → /private/var, so compare via EvalSymlinks
	expected, _ := filepath.EvalSymlinks(filepath.Join(repoPath, ".env"))
	actual, _ := filepath.EvalSymlinks(target)
	assert.Equal(t, expected, actual)

	require.NoError(t, worktree.Delete(r, cfg, worktree.DeleteOpts{
		Name:         "symlink-test",
		DeleteBranch: true,
	}))
}

func TestIntegration_CreateExistingBranch(t *testing.T) {
	_, r := initTestRepo(t)
	wtBase := t.TempDir()

	cfg := &config.Config{
		WorktreeBase:  wtBase,
		DefaultBranch: "main",
	}

	// Create a branch without a worktree
	_, err := r.Run("git", "branch", "existing-branch")
	require.NoError(t, err)

	// Create worktree for the existing branch — should reuse, not create new
	wt, err := worktree.Create(r, cfg, worktree.CreateOpts{
		Name:      "existing-branch",
		SkipSetup: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "existing-branch", wt.Branch)
	assert.DirExists(t, wt.Path)

	require.NoError(t, worktree.Delete(r, cfg, worktree.DeleteOpts{
		Name:         "existing-branch",
		DeleteBranch: true,
	}))
}
