package worktree

import (
	"os"
	"path/filepath"

	"github.com/davidmks/sarj/internal/exec"
)

// trashDirName is the folder, next to the worktrees, that holds removed
// worktrees until a background process deletes their files.
const trashDirName = ".sarj-trash"

// moveToTrash renames the worktree directory into the trash folder next to it
// and returns that trash folder. A rename is instant however many files the
// worktree holds, while deleting them one by one takes seconds for large
// dependency folders. Keeping the trash next to the worktree keeps it on the
// same filesystem; the rename fails otherwise.
func moveToTrash(path string) (string, error) {
	trash := filepath.Join(filepath.Dir(path), trashDirName)
	if err := os.MkdirAll(trash, 0o750); err != nil {
		return "", err
	}
	// A unique parent folder keeps two deletes of the same name from colliding.
	dir, err := os.MkdirTemp(trash, filepath.Base(path)+"-")
	if err != nil {
		return "", err
	}
	if err := os.Rename(path, filepath.Join(dir, filepath.Base(path))); err != nil {
		os.Remove(dir) //nolint:errcheck
		return "", err
	}
	return trash, nil
}

// purgeTrash starts a background rm of every entry in the trash folder and
// returns without waiting. Including older entries clears leftovers from an
// earlier purge that was cut off, or files a process wrote after rm passed.
//
// The trash folder itself is kept: a delete running right after this one
// renames into it, and removing it would race that rename.
func purgeTrash(r exec.Runner, trash string) error {
	entries, err := os.ReadDir(trash)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	args := []string{"-rf", "--"}
	for _, e := range entries {
		args = append(args, filepath.Join(trash, e.Name()))
	}
	return r.StartDetached("rm", args...)
}
