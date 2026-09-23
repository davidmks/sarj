package worktree

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/davidmks/sarj/internal/exec"
)

// trashDirName is the folder, next to the worktrees, that holds removed
// worktrees until a background process deletes their files.
const trashDirName = ".sarj-trash"

// moveToTrash renames the worktree directory into the trash folder next to it
// and returns its new path. A rename is instant however many files the
// worktree holds, while deleting them one by one takes seconds for large
// dependency folders. Keeping the trash next to the worktree keeps it on the
// same filesystem; the rename fails otherwise.
//
// The worktree moves straight to a unique name. Moving it into a new empty
// folder instead would race a running purge, which could delete that folder
// before the rename lands.
func moveToTrash(path string) (string, error) {
	trash := filepath.Join(filepath.Dir(path), trashDirName)
	if err := os.MkdirAll(trash, 0o750); err != nil {
		return "", err
	}
	if err := requireRealDir(trash); err != nil {
		return "", err
	}
	moved := filepath.Join(trash, filepath.Base(path)+"-"+strings.ToLower(rand.Text()))
	if err := os.Rename(path, moved); err != nil {
		return "", err
	}
	return moved, nil
}

// requireRealDir returns an error unless path is a directory itself, not a
// symlink to one, so the trash can never point at an unrelated folder.
func requireRealDir(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}
	return nil
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
//
// It refuses any folder not named .sarj-trash, and a .sarj-trash that is a
// symlink or a file, so a wrong argument to the hidden command cannot empty
// an unrelated folder.
func PurgeTrash(trash string) error {
	if filepath.Base(filepath.Clean(trash)) != trashDirName {
		return fmt.Errorf("refusing to purge %s: not a %s folder", trash, trashDirName)
	}
	if err := requireRealDir(trash); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("refusing to purge %s: %w", trash, err)
	}
	entries, err := os.ReadDir(trash)
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
