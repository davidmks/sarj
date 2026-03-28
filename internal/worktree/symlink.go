package worktree

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// CreateSymlinks creates symlinks in the worktree pointing back to files/dirs
// in the main repo. Skips entries that don't exist in the main repo.
func CreateSymlinks(mainRepo, wtPath string, links []string) error {
	for _, link := range links {
		src := filepath.Join(mainRepo, link)
		dst := filepath.Join(wtPath, link)

		// Skip if source doesn't exist in main repo
		if _, err := os.Stat(src); errors.Is(err, fs.ErrNotExist) {
			continue
		}

		// Ensure parent directory exists (for paths like .claude/settings.local.json)
		if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
			return fmt.Errorf("creating parent dir for symlink %s: %w", link, err)
		}

		// Remove existing file/symlink at destination so ln -sf semantics work
		os.Remove(dst) //nolint:errcheck

		if err := os.Symlink(src, dst); err != nil {
			return fmt.Errorf("symlinking %s: %w", link, err)
		}
	}

	return nil
}
