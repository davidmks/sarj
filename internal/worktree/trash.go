package worktree

import (
	"errors"
	"io/fs"
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

// PurgeCommand is the hidden sarj subcommand that runs PurgeTrash. Delete
// starts sarj with it in the background, because the files must outlive the
// sarj process: deleting from inside the worktree's tmux session kills it.
const PurgeCommand = "__purge-trash"

// startPurge starts sarj again as a detached process that runs PurgeTrash on
// trash, and returns without waiting.
func startPurge(r exec.Runner, trash string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return r.StartDetached(exe, PurgeCommand, trash)
}

// PurgeTrash deletes every entry in the trash folder and keeps the folder
// itself, because another delete may be renaming into it. Deleting all
// entries, not only the newest, also clears leftovers from a purge that was
// cut off, for example by a reboot. A missing trash folder is not an error.
//
// Several purges may run on the same folder at once. os.RemoveAll handles
// that: it treats files that vanish mid-walk as deleted. rm does not. BSD rm
// stops at the first vanished folder and silently skips its other
// arguments, and uutils rm leaves files behind.
func PurgeTrash(trash string) error {
	entries, err := os.ReadDir(trash)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(trash, e.Name())); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
