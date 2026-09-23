package worktree_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/davidmks/sarj/internal/config"
	"github.com/davidmks/sarj/internal/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRunner struct {
	calls          []string
	responses      map[string]response
	interactiveErr error
}

type response struct {
	out string
	err error
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	call := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, call)

	parts := strings.Fields(call)
	for i := len(parts); i > 0; i-- {
		key := strings.Join(parts[:i], " ")
		if resp, ok := f.responses[key]; ok {
			return resp.out, resp.err
		}
	}
	return "", nil
}

func (f *fakeRunner) RunWithEnv(ctx context.Context, _ []string, name string, args ...string) (string, error) {
	return f.Run(ctx, name, args...)
}

func (f *fakeRunner) RunInteractive(_ context.Context, _ string, _ ...string) error {
	return f.interactiveErr
}

func (f *fakeRunner) StartDetached(name string, args ...string) error {
	_, err := f.Run(context.Background(), name, args...)
	return err
}

func (f *fakeRunner) hasCall(substr string) bool {
	for _, c := range f.calls {
		if strings.Contains(c, substr) {
			return true
		}
	}
	return false
}

func TestCreate_NewBranch(t *testing.T) {
	wtBase := t.TempDir()
	cfg := &config.Config{WorktreeBase: wtBase, DefaultBranch: "main"}
	r := &fakeRunner{responses: map[string]response{
		"git fetch": {},
		"git show-ref --verify --quiet refs/heads/my-feature":    {err: fmt.Errorf("not found")},
		"git show-ref --verify --quiet refs/remotes/origin/main": {},
		"git worktree": {},
	}}

	wt, err := worktree.Create(t.Context(), r, cfg, worktree.CreateOpts{
		Name:      "my-feature",
		SkipSetup: true,
	})

	require.NoError(t, err)
	assert.Equal(t, "my-feature", wt.Branch)
	assert.Equal(t, filepath.Join(wtBase, "my-feature"), wt.Path)
	assert.True(t, r.hasCall(wtBase+"/my-feature origin/main"))
}

func TestCreate_ExistingBranch(t *testing.T) {
	wtBase := t.TempDir()
	cfg := &config.Config{WorktreeBase: wtBase, DefaultBranch: "main"}
	r := &fakeRunner{responses: map[string]response{
		"git fetch": {},
		"git show-ref --verify --quiet refs/heads/existing-branch": {},
		"git worktree": {},
		"git rev-list": {out: "0"},
	}}

	wt, err := worktree.Create(t.Context(), r, cfg, worktree.CreateOpts{
		Name:      "existing-branch",
		SkipSetup: true,
	})

	require.NoError(t, err)
	assert.Equal(t, "existing-branch", wt.Branch)
	assert.True(t, r.hasCall("worktree add"))
	// " -b " distinguishes the flag from substrings like "existing-branch"
	assert.False(t, r.hasCall(" -b "))
}

func TestCreate_GeneratesName(t *testing.T) {
	wtBase := t.TempDir()
	cfg := &config.Config{WorktreeBase: wtBase, DefaultBranch: "main"}
	r := &fakeRunner{responses: map[string]response{
		"git fetch": {},
		"git show-ref --verify --quiet refs/heads/":   {err: fmt.Errorf("not found")},
		"git show-ref --verify --quiet refs/remotes/": {},
		"git worktree": {},
	}}

	wt, err := worktree.Create(t.Context(), r, cfg, worktree.CreateOpts{SkipSetup: true})

	require.NoError(t, err)
	assert.NotEmpty(t, wt.Branch)
}

func TestCreate_ExistingDir(t *testing.T) {
	wtBase := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(wtBase, "existing"), 0o750))
	cfg := &config.Config{WorktreeBase: wtBase}
	r := &fakeRunner{}

	_, err := worktree.Create(t.Context(), r, cfg, worktree.CreateOpts{
		Name:      "existing",
		SkipSetup: true,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, worktree.ErrWorktreeExists)
}

func TestCreate_FetchFailsContinues(t *testing.T) {
	wtBase := t.TempDir()
	cfg := &config.Config{WorktreeBase: wtBase, DefaultBranch: "main"}
	r := &fakeRunner{responses: map[string]response{
		"git fetch":    {err: fmt.Errorf("network error")},
		"git show-ref": {err: fmt.Errorf("not found")},
		"git worktree": {},
	}}

	wt, err := worktree.Create(t.Context(), r, cfg, worktree.CreateOpts{
		Name:      "offline",
		SkipSetup: true,
	})

	require.NoError(t, err)
	assert.Equal(t, "offline", wt.Branch)
	assert.True(t, r.hasCall(wtBase+"/offline main"))
}

func TestCreate_RollbackOnSetupFailure(t *testing.T) {
	wtBase := t.TempDir()
	cfg := &config.Config{
		WorktreeBase:  wtBase,
		DefaultBranch: "main",
		SetupCommand:  "make setup",
	}
	r := &fakeRunner{
		responses: map[string]response{
			"git fetch": {},
			"git show-ref --verify --quiet refs/heads/doomed":        {err: fmt.Errorf("not found")},
			"git show-ref --verify --quiet refs/remotes/origin/main": {},
			"git worktree": {},
		},
		interactiveErr: fmt.Errorf("setup failed"),
	}

	_, err := worktree.Create(t.Context(), r, cfg, worktree.CreateOpts{Name: "doomed"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "setup command failed")
	assert.True(t, r.hasCall("worktree remove --force"))
	assert.True(t, r.hasCall("branch -D doomed"))
}

func TestCreate_RollbackKeepsBranchWhenPreexisting(t *testing.T) {
	wtBase := t.TempDir()
	cfg := &config.Config{
		WorktreeBase:  wtBase,
		DefaultBranch: "main",
		SetupCommand:  "make setup",
	}
	r := &fakeRunner{
		responses: map[string]response{
			"git fetch": {},
			"git show-ref --verify --quiet refs/heads/preexisting": {},
			"git worktree": {},
			"git rev-list": {out: "0"},
		},
		interactiveErr: fmt.Errorf("setup failed"),
	}

	_, err := worktree.Create(t.Context(), r, cfg, worktree.CreateOpts{Name: "preexisting"})

	require.Error(t, err)
	assert.True(t, r.hasCall("worktree remove --force"))
	assert.False(t, r.hasCall("branch -D"))
}

func TestCreate_NewBranch_FallsBackToLocal(t *testing.T) {
	wtBase := t.TempDir()
	cfg := &config.Config{WorktreeBase: wtBase, DefaultBranch: "main"}
	r := &fakeRunner{responses: map[string]response{
		"git fetch":    {},
		"git show-ref": {err: fmt.Errorf("not found")},
		"git worktree": {},
	}}

	wt, err := worktree.Create(t.Context(), r, cfg, worktree.CreateOpts{
		Name:      "my-feature",
		SkipSetup: true,
	})

	require.NoError(t, err)
	assert.Equal(t, "my-feature", wt.Branch)
	assert.True(t, r.hasCall(wtBase+"/my-feature main"))
}

func TestCreate_NewBranch_BaseAlreadyRemoteRef(t *testing.T) {
	wtBase := t.TempDir()
	cfg := &config.Config{WorktreeBase: wtBase, DefaultBranch: "main"}
	r := &fakeRunner{responses: map[string]response{
		"git fetch": {},
		"git show-ref --verify --quiet refs/heads/my-feature": {err: fmt.Errorf("not found")},
		"git worktree": {},
	}}

	wt, err := worktree.Create(t.Context(), r, cfg, worktree.CreateOpts{
		Name:      "my-feature",
		Base:      "origin/develop",
		SkipSetup: true,
	})

	require.NoError(t, err)
	assert.Equal(t, "my-feature", wt.Branch)
	assert.True(t, r.hasCall(wtBase+"/my-feature origin/develop"))
}

func TestCreate_ExistingBranch_BehindWarning(t *testing.T) {
	wtBase := t.TempDir()
	cfg := &config.Config{WorktreeBase: wtBase, DefaultBranch: "main"}
	var buf bytes.Buffer
	r := &fakeRunner{responses: map[string]response{
		"git fetch": {},
		"git show-ref --verify --quiet refs/heads/stale-branch": {},
		"git worktree": {},
		"git rev-list": {out: "3"},
	}}

	_, err := worktree.Create(t.Context(), r, cfg, worktree.CreateOpts{
		Name:      "stale-branch",
		SkipSetup: true,
		Progress:  &buf,
	})

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "warning: branch stale-branch is 3 commit(s) behind origin/stale-branch")
}

func TestCreate_ExistingBranch_NotBehind(t *testing.T) {
	wtBase := t.TempDir()
	cfg := &config.Config{WorktreeBase: wtBase, DefaultBranch: "main"}
	var buf bytes.Buffer
	r := &fakeRunner{responses: map[string]response{
		"git fetch": {},
		"git show-ref --verify --quiet refs/heads/up-to-date": {},
		"git worktree": {},
		"git rev-list": {out: "0"},
	}}

	_, err := worktree.Create(t.Context(), r, cfg, worktree.CreateOpts{
		Name:      "up-to-date",
		SkipSetup: true,
		Progress:  &buf,
	})

	require.NoError(t, err)
	assert.NotContains(t, buf.String(), "warning: branch")
}

func TestCreate_ExistingBranch_NoRemoteCounterpart(t *testing.T) {
	wtBase := t.TempDir()
	cfg := &config.Config{WorktreeBase: wtBase, DefaultBranch: "main"}
	var buf bytes.Buffer
	r := &fakeRunner{responses: map[string]response{
		"git fetch": {},
		"git show-ref --verify --quiet refs/heads/local-only": {},
		"git worktree": {},
		"git rev-list": {err: fmt.Errorf("unknown revision")},
	}}

	_, err := worktree.Create(t.Context(), r, cfg, worktree.CreateOpts{
		Name:      "local-only",
		SkipSetup: true,
		Progress:  &buf,
	})

	require.NoError(t, err)
	assert.NotContains(t, buf.String(), "warning: branch")
}

func TestCreate_FetchFailsFallsBackToLocal(t *testing.T) {
	wtBase := t.TempDir()
	cfg := &config.Config{WorktreeBase: wtBase, DefaultBranch: "main"}
	r := &fakeRunner{responses: map[string]response{
		"git fetch":    {err: fmt.Errorf("network error")},
		"git show-ref": {err: fmt.Errorf("not found")},
		"git worktree": {},
	}}

	wt, err := worktree.Create(t.Context(), r, cfg, worktree.CreateOpts{
		Name:      "offline-new",
		SkipSetup: true,
	})

	require.NoError(t, err)
	assert.Equal(t, "offline-new", wt.Branch)
	assert.True(t, r.hasCall(wtBase+"/offline-new main"))
}

// purgeTarget returns the trash folder passed to the background purge, or ""
// when Delete did not start one.
func purgeTarget(r *fakeRunner) string {
	for _, c := range r.calls {
		if _, trash, ok := strings.Cut(c, " "+worktree.PurgeCommand+" "); ok {
			return trash
		}
	}
	return ""
}

// trashEntries returns the full paths of the entries in base's trash folder.
func trashEntries(t *testing.T, base string) []string {
	t.Helper()
	trash := filepath.Join(base, ".sarj-trash")
	entries, err := os.ReadDir(trash)
	require.NoError(t, err)
	paths := make([]string, len(entries))
	for i, e := range entries {
		paths[i] = filepath.Join(trash, e.Name())
	}
	return paths
}

func TestDelete(t *testing.T) {
	base := t.TempDir()
	wtPath := filepath.Join(base, "my-feature")
	require.NoError(t, os.MkdirAll(wtPath, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(wtPath, "file.txt"), []byte("x"), 0o600))
	r := &fakeRunner{responses: map[string]response{"git worktree": {}}}

	err := worktree.Delete(t.Context(), r, worktree.DeleteOpts{Path: wtPath})

	require.NoError(t, err)
	assert.NoDirExists(t, wtPath)
	assert.False(t, r.hasCall("worktree remove"), "should move to trash instead of removing in place")
	assert.True(t, r.hasCall("worktree prune"))

	entries := trashEntries(t, base)
	require.Len(t, entries, 1)
	assert.FileExists(t, filepath.Join(entries[0], "my-feature", "file.txt"))
	assert.Equal(t, filepath.Join(base, ".sarj-trash"), purgeTarget(r))
}

func TestDelete_Locked(t *testing.T) {
	base := t.TempDir()
	wtPath := filepath.Join(base, "locked-wt")
	require.NoError(t, os.MkdirAll(wtPath, 0o750))
	r := &fakeRunner{responses: map[string]response{
		"git worktree remove": {err: fmt.Errorf("cannot remove a locked working tree")},
	}}

	err := worktree.Delete(t.Context(), r, worktree.DeleteOpts{Path: wtPath, Locked: true})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "removing worktree")
	assert.DirExists(t, wtPath)
	assert.NoDirExists(t, filepath.Join(base, ".sarj-trash"))
	assert.Empty(t, purgeTarget(r))
}

func TestDelete_TrashFails(t *testing.T) {
	base := t.TempDir()
	wtPath := filepath.Join(base, "my-feature")
	require.NoError(t, os.MkdirAll(wtPath, 0o750))
	// A regular file where the trash folder should be makes the move fail.
	require.NoError(t, os.WriteFile(filepath.Join(base, ".sarj-trash"), nil, 0o600))
	r := &fakeRunner{responses: map[string]response{"git worktree": {}}}
	var buf bytes.Buffer

	err := worktree.Delete(t.Context(), r, worktree.DeleteOpts{Path: wtPath, Progress: &buf})

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "could not move worktree to trash")
	assert.True(t, r.hasCall("worktree remove"))
	assert.True(t, r.hasCall("worktree prune"))
	assert.Empty(t, purgeTarget(r))
}

func TestDelete_PruneFailsAfterTrash(t *testing.T) {
	base := t.TempDir()
	wtPath := filepath.Join(base, "my-feature")
	require.NoError(t, os.MkdirAll(wtPath, 0o750))
	r := &fakeRunner{responses: map[string]response{
		"git worktree prune": {err: fmt.Errorf("prune failed")},
	}}

	err := worktree.Delete(t.Context(), r, worktree.DeleteOpts{Path: wtPath})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "pruning worktree")
	assert.Empty(t, purgeTarget(r))
}

func TestPurgeTrash(t *testing.T) {
	trash := filepath.Join(t.TempDir(), ".sarj-trash")
	// A fresh entry and a leftover from an earlier purge that was cut off.
	for _, dir := range []string{"new-123/new/node_modules/pkg", "old-456/old"} {
		require.NoError(t, os.MkdirAll(filepath.Join(trash, dir), 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(trash, dir, "f"), []byte("x"), 0o600))
	}

	require.NoError(t, worktree.PurgeTrash(trash))

	assert.DirExists(t, trash, "a delete may be renaming into it")
	entries, err := os.ReadDir(trash)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestPurgeTrash_MissingFolder(t *testing.T) {
	assert.NoError(t, worktree.PurgeTrash(filepath.Join(t.TempDir(), ".sarj-trash")))
}

// TestPurgeTrash_Concurrent runs several purges on the same trash at once,
// as a bulk delete does. Each one must finish every entry even when another
// purge deletes files under it.
func TestPurgeTrash_Concurrent(t *testing.T) {
	trash := filepath.Join(t.TempDir(), ".sarj-trash")
	for i := range 5 {
		for j := range 50 {
			dir := filepath.Join(trash, fmt.Sprintf("wt%d-x", i), fmt.Sprintf("wt%d", i), fmt.Sprintf("pkg%d", j), "lib")
			require.NoError(t, os.MkdirAll(dir, 0o750))
			for k := range 5 {
				require.NoError(t, os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d", k)), nil, 0o600))
			}
		}
	}

	var wg sync.WaitGroup
	errs := make([]error, 5)
	for i := range 5 {
		wg.Go(func() { errs[i] = worktree.PurgeTrash(trash) })
	}
	wg.Wait()

	for _, err := range errs {
		assert.NoError(t, err)
	}
	entries, err := os.ReadDir(trash)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestDelete_SameNameTwice(t *testing.T) {
	base := t.TempDir()
	wtPath := filepath.Join(base, "my-feature")
	r := &fakeRunner{responses: map[string]response{"git worktree": {}}}

	for range 2 {
		require.NoError(t, os.MkdirAll(wtPath, 0o750))
		require.NoError(t, worktree.Delete(t.Context(), r, worktree.DeleteOpts{Path: wtPath}))
	}

	assert.Len(t, trashEntries(t, base), 2)
	assert.False(t, r.hasCall("worktree remove"), "second delete should not fall back")
}

func TestDelete_StaleEntry(t *testing.T) {
	wtPath := filepath.Join(t.TempDir(), "gone-wt")

	r := &fakeRunner{responses: map[string]response{
		"git worktree": {},
	}}

	err := worktree.Delete(t.Context(), r, worktree.DeleteOpts{Path: wtPath})

	require.NoError(t, err)
	assert.False(t, r.hasCall("worktree remove"), "should skip remove for missing directory")
	assert.True(t, r.hasCall("worktree prune"))
}

func TestRename(t *testing.T) {
	base := t.TempDir()
	r := &fakeRunner{responses: map[string]response{"git": {}}}

	err := worktree.Rename(t.Context(), r, worktree.RenameOpts{
		OldBranch: "my-feature",
		NewBranch: "feat/new",
		OldPath:   filepath.Join(base, "my-feature"),
		NewPath:   filepath.Join(base, "feat-new"),
	})

	require.NoError(t, err)
	assert.Equal(t, []string{
		"git branch -m my-feature feat/new",
		"git worktree move " + filepath.Join(base, "my-feature") + " " + filepath.Join(base, "feat-new"),
	}, r.calls, "branch must be renamed before the directory moves")
}

func TestRename_KeepsPath(t *testing.T) {
	tests := []struct {
		name    string
		oldPath string
		newPath string
	}{
		{name: "no destination", oldPath: "/wt/my-feature", newPath: ""},
		{name: "destination unchanged", oldPath: "/wt/feat-foo", newPath: "/wt/feat-foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{responses: map[string]response{"git": {}}}

			err := worktree.Rename(t.Context(), r, worktree.RenameOpts{
				OldBranch: "my-feature",
				NewBranch: "feat/new",
				OldPath:   tt.oldPath,
				NewPath:   tt.newPath,
			})

			require.NoError(t, err)
			assert.True(t, r.hasCall("branch -m my-feature feat/new"))
			assert.False(t, r.hasCall("worktree move"))
		})
	}
}

func TestRename_BranchFails(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"git branch -m": {err: fmt.Errorf("branch exists")},
	}}

	err := worktree.Rename(t.Context(), r, worktree.RenameOpts{
		OldBranch: "my-feature",
		NewBranch: "feat/new",
		OldPath:   "/wt/my-feature",
		NewPath:   "/wt/feat-new",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "renaming branch my-feature")
	assert.False(t, r.hasCall("worktree move"))
}

func TestRename_MoveFailureRollsBackBranch(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"git worktree move": {err: fmt.Errorf("destination busy")},
	}}

	err := worktree.Rename(t.Context(), r, worktree.RenameOpts{
		OldBranch: "my-feature",
		NewBranch: "feat/new",
		OldPath:   "/wt/my-feature",
		NewPath:   "/wt/feat-new",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "moving worktree")
	assert.True(t, r.hasCall("branch -m feat/new my-feature"), "branch rename should be undone")
}

func TestList_Error(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {err: fmt.Errorf("not a git repo")},
	}}

	_, err := worktree.List(t.Context(), r)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing worktrees")
}

func TestList(t *testing.T) {
	porcelain := "worktree /repo\nHEAD abc\nbranch refs/heads/main\n\nworktree /wt/feat\nHEAD def\nbranch refs/heads/feat\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree": {out: porcelain},
	}}

	wts, err := worktree.List(t.Context(), r)

	require.NoError(t, err)
	assert.Len(t, wts, 2)
	assert.Equal(t, "main", wts[0].Branch)
	assert.Equal(t, "feat", wts[1].Branch)
}
