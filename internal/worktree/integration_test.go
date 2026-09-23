//go:build integration

package worktree_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/davidmks/sarj/internal/config"
	"github.com/davidmks/sarj/internal/exec"
	"github.com/davidmks/sarj/internal/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain lets the test binary stand in for sarj. A real Delete starts
// os.Executable() with the purge command in the background, and in tests that
// is this binary. Without this, it would run the whole test suite again.
func TestMain(m *testing.M) {
	if len(os.Args) == 3 && os.Args[1] == worktree.PurgeCommand {
		if err := worktree.PurgeTrash(os.Args[2]); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

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
		_, err := runner.Run(t.Context(), cmd[0], cmd[1:]...)
		require.NoError(t, err)
	}

	require.NoError(t, os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("# test"), 0o600))
	_, err := runner.Run(t.Context(), "git", "add", ".")
	require.NoError(t, err)
	_, err = runner.Run(t.Context(), "git", "commit", "-m", "init")
	require.NoError(t, err)

	return repoPath, runner
}

// requireTrashEmpties waits for the background purge started by Delete to clear
// the trash folder in wtBase.
func requireTrashEmpties(t *testing.T, wtBase string) {
	t.Helper()
	trash := filepath.Join(wtBase, ".sarj-trash")
	require.Eventually(t, func() bool {
		entries, err := os.ReadDir(trash)
		return err == nil && len(entries) == 0
	}, 10*time.Second, 20*time.Millisecond, "background purge should empty %s", trash)
}

func TestIntegration_CreateListDelete(t *testing.T) {
	_, r := initTestRepo(t)
	wtBase := t.TempDir()

	cfg := &config.Config{
		WorktreeBase:  wtBase,
		DefaultBranch: "main",
	}

	wt, err := worktree.Create(t.Context(), r, cfg, worktree.CreateOpts{
		Name:      "test-branch",
		SkipSetup: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "test-branch", wt.Branch)
	assert.DirExists(t, wt.Path)

	wts, err := worktree.List(t.Context(), r)
	require.NoError(t, err)
	assert.Len(t, wts, 2)

	err = worktree.Delete(t.Context(), r, worktree.DeleteOpts{Path: wt.Path})
	require.NoError(t, err)
	assert.NoDirExists(t, wt.Path)
	requireTrashEmpties(t, wtBase)

	wts, err = worktree.List(t.Context(), r)
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

	wt, err := worktree.Create(t.Context(), r, cfg, worktree.CreateOpts{
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

	require.NoError(t, worktree.Delete(t.Context(), r, worktree.DeleteOpts{Path: wt.Path}))
	requireTrashEmpties(t, wtBase)
	assert.FileExists(t, filepath.Join(repoPath, ".env"), "purge must not follow symlinks into the main repo")
}

// TestIntegration_DeleteFreesBranchAndName covers what the user does right
// after a delete: delete the branch, or create a worktree with the same name.
// Both only work once git has forgotten the moved worktree.
func TestIntegration_DeleteFreesBranchAndName(t *testing.T) {
	_, r := initTestRepo(t)
	wtBase := t.TempDir()
	cfg := &config.Config{WorktreeBase: wtBase, DefaultBranch: "main"}

	wt, err := worktree.Create(t.Context(), r, cfg, worktree.CreateOpts{Name: "reuse-me", SkipSetup: true})
	require.NoError(t, err)
	require.NoError(t, worktree.Delete(t.Context(), r, worktree.DeleteOpts{Path: wt.Path}))

	_, err = r.Run(t.Context(), "git", "branch", "-D", "reuse-me")
	require.NoError(t, err, "branch should no longer count as checked out")

	wt, err = worktree.Create(t.Context(), r, cfg, worktree.CreateOpts{Name: "reuse-me", SkipSetup: true})
	require.NoError(t, err, "path should be free right after delete")
	assert.DirExists(t, wt.Path)

	require.NoError(t, worktree.Delete(t.Context(), r, worktree.DeleteOpts{Path: wt.Path}))
	requireTrashEmpties(t, wtBase)
}

func TestIntegration_Rename(t *testing.T) {
	_, r := initTestRepo(t)
	wtBase := t.TempDir()

	cfg := &config.Config{
		WorktreeBase:  wtBase,
		DefaultBranch: "main",
	}

	wt, err := worktree.Create(t.Context(), r, cfg, worktree.CreateOpts{
		Name:      "feat/old",
		SkipSetup: true,
	})
	require.NoError(t, err)

	// Uncommitted work must survive the move — that is the whole point of
	// renaming after planning rather than before.
	scratch := filepath.Join(wt.Path, "scratch.txt")
	require.NoError(t, os.WriteFile(scratch, []byte("work in progress"), 0o600))

	newPath := filepath.Join(wtBase, "feat-new")
	require.NoError(t, worktree.Rename(t.Context(), r, worktree.RenameOpts{
		OldBranch: "feat/old",
		NewBranch: "feat/new",
		OldPath:   wt.Path,
		NewPath:   newPath,
	}))

	assert.NoDirExists(t, wt.Path)
	assert.DirExists(t, newPath)

	moved, err := os.ReadFile(filepath.Join(newPath, "scratch.txt"))
	require.NoError(t, err)
	assert.Equal(t, "work in progress", string(moved))

	branches, err := r.Run(t.Context(), "git", "branch", "--list", "--format=%(refname:short)")
	require.NoError(t, err)
	assert.Contains(t, branches, "feat/new")
	assert.NotContains(t, branches, "feat/old")

	wts, err := worktree.List(t.Context(), r)
	require.NoError(t, err)
	found := worktree.FindByName(wts, "feat/new")
	require.NotNil(t, found, "renamed worktree should still resolve by name")
	assert.Equal(t, "feat/new", found.Branch)

	require.NoError(t, worktree.Delete(t.Context(), r, worktree.DeleteOpts{Path: newPath}))
	requireTrashEmpties(t, wtBase)
}

func TestIntegration_CreateExistingBranch(t *testing.T) {
	_, r := initTestRepo(t)
	wtBase := t.TempDir()

	cfg := &config.Config{
		WorktreeBase:  wtBase,
		DefaultBranch: "main",
	}

	// Create a branch without a worktree
	_, err := r.Run(t.Context(), "git", "branch", "existing-branch")
	require.NoError(t, err)

	wt, err := worktree.Create(t.Context(), r, cfg, worktree.CreateOpts{
		Name:      "existing-branch",
		SkipSetup: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "existing-branch", wt.Branch)
	assert.DirExists(t, wt.Path)

	require.NoError(t, worktree.Delete(t.Context(), r, worktree.DeleteOpts{Path: wt.Path}))
	requireTrashEmpties(t, wtBase)
}
