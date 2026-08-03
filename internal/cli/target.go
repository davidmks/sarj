package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/davidmks/sarj/internal/worktree"
)

// errMainWorktree is returned when a command targets the main worktree, which
// can be neither renamed nor deleted.
var errMainWorktree = errors.New("cannot use the main worktree")

// target is a worktree a command was pointed at, plus the name the user
// referred to it by.
type target struct {
	wt   *worktree.Worktree
	name string
	// inferred reports that the target came from the current directory rather
	// than an argument. Commands decide for themselves whether that warrants a
	// confirmation prompt.
	inferred bool
}

// resolveTargets resolves all targets up front so unknown names fail before any
// side effect. Zero args falls back to cwd inference.
func resolveTargets(wts []worktree.Worktree, args []string) ([]target, error) {
	if len(args) == 0 {
		t, err := resolveFromCwd(wts)
		if err != nil {
			return nil, err
		}
		return []target{t}, nil
	}
	return resolveNamed(wts, args)
}

func resolveNamed(wts []worktree.Worktree, args []string) ([]target, error) {
	mainPath := worktree.MainPath(wts)
	targets := make([]target, 0, len(args))
	var unknown []string
	for _, name := range args {
		wt := worktree.FindByName(wts, name)
		if wt == nil {
			unknown = append(unknown, name)
			continue
		}
		if wt.Path == mainPath {
			return nil, errMainWorktree
		}
		targets = append(targets, target{wt: wt, name: name})
	}
	if len(unknown) > 0 {
		return nil, fmt.Errorf("worktree not found: %s", strings.Join(unknown, ", "))
	}
	return targets, nil
}

func resolveFromCwd(wts []worktree.Worktree) (target, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return target{}, fmt.Errorf("getting current directory: %w", err)
	}
	cwd, err = filepath.EvalSymlinks(cwd)
	if err != nil {
		return target{}, fmt.Errorf("resolving current directory: %w", err)
	}
	wt := worktree.FindByPath(wts, cwd)
	if wt == nil {
		return target{}, fmt.Errorf("current directory is not inside a worktree")
	}
	if wt.Path == worktree.MainPath(wts) {
		return target{}, errMainWorktree
	}
	return target{wt: wt, name: filepath.Base(wt.Path), inferred: true}, nil
}
