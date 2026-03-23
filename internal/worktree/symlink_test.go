package worktree_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/davidmks/sarj/internal/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateSymlinks(t *testing.T) {
	t.Run("creates symlinks for existing files", func(t *testing.T) {
		mainRepo := t.TempDir()
		wtPath := t.TempDir()

		require.NoError(t, os.WriteFile(filepath.Join(mainRepo, ".env"), []byte("SECRET=x"), 0o644))

		err := worktree.CreateSymlinks(mainRepo, wtPath, []string{".env"})

		require.NoError(t, err)
		target, err := os.Readlink(filepath.Join(wtPath, ".env"))
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(mainRepo, ".env"), target)
	})

	t.Run("skips missing source files", func(t *testing.T) {
		mainRepo := t.TempDir()
		wtPath := t.TempDir()

		err := worktree.CreateSymlinks(mainRepo, wtPath, []string{".env", "nope.txt"})

		require.NoError(t, err)
		_, err = os.Lstat(filepath.Join(wtPath, "nope.txt"))
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("creates parent directories", func(t *testing.T) {
		mainRepo := t.TempDir()
		wtPath := t.TempDir()

		nested := filepath.Join(mainRepo, ".claude")
		require.NoError(t, os.MkdirAll(nested, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(nested, "settings.local.json"), []byte("{}"), 0o644))

		err := worktree.CreateSymlinks(mainRepo, wtPath, []string{".claude/settings.local.json"})

		require.NoError(t, err)
		target, err := os.Readlink(filepath.Join(wtPath, ".claude", "settings.local.json"))
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(mainRepo, ".claude", "settings.local.json"), target)
	})

	t.Run("handles directories", func(t *testing.T) {
		mainRepo := t.TempDir()
		wtPath := t.TempDir()

		require.NoError(t, os.MkdirAll(filepath.Join(mainRepo, "ssl"), 0o755))

		err := worktree.CreateSymlinks(mainRepo, wtPath, []string{"ssl"})

		require.NoError(t, err)
		info, err := os.Lstat(filepath.Join(wtPath, "ssl"))
		require.NoError(t, err)
		assert.NotZero(t, info.Mode()&os.ModeSymlink)
	})

	t.Run("replaces existing file at destination", func(t *testing.T) {
		mainRepo := t.TempDir()
		wtPath := t.TempDir()

		require.NoError(t, os.WriteFile(filepath.Join(mainRepo, ".env"), []byte("real"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(wtPath, ".env"), []byte("old"), 0o644))

		err := worktree.CreateSymlinks(mainRepo, wtPath, []string{".env"})

		require.NoError(t, err)
		content, err := os.ReadFile(filepath.Join(wtPath, ".env"))
		require.NoError(t, err)
		assert.Equal(t, "real", string(content))
	})
}
