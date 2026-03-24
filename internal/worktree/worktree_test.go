package worktree_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func (f *fakeRunner) Run(name string, args ...string) (string, error) {
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

func (f *fakeRunner) RunInteractive(_ string, _ ...string) error {
	return f.interactiveErr
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
		"git fetch":    {},
		"git show-ref": {err: fmt.Errorf("not found")},
		"git worktree": {},
	}}

	wt, err := worktree.Create(r, cfg, worktree.CreateOpts{
		Name:      "my-feature",
		SkipSetup: true,
	})

	require.NoError(t, err)
	assert.Equal(t, "my-feature", wt.Branch)
	assert.Equal(t, filepath.Join(wtBase, "my-feature"), wt.Path)
}

func TestCreate_ExistingBranch(t *testing.T) {
	wtBase := t.TempDir()
	cfg := &config.Config{WorktreeBase: wtBase, DefaultBranch: "main"}
	r := &fakeRunner{responses: map[string]response{
		"git fetch":    {},
		"git show-ref": {},
		"git worktree": {},
	}}

	wt, err := worktree.Create(r, cfg, worktree.CreateOpts{
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
		"git fetch":    {},
		"git show-ref": {err: fmt.Errorf("not found")},
		"git worktree": {},
	}}

	wt, err := worktree.Create(r, cfg, worktree.CreateOpts{SkipSetup: true})

	require.NoError(t, err)
	assert.NotEmpty(t, wt.Branch)
}

func TestCreate_ExistingDir(t *testing.T) {
	wtBase := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(wtBase, "existing"), 0o750))
	cfg := &config.Config{WorktreeBase: wtBase}
	r := &fakeRunner{}

	_, err := worktree.Create(r, cfg, worktree.CreateOpts{
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

	wt, err := worktree.Create(r, cfg, worktree.CreateOpts{
		Name:      "offline",
		SkipSetup: true,
	})

	require.NoError(t, err)
	assert.Equal(t, "offline", wt.Branch)
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
			"git fetch":    {},
			"git show-ref": {err: fmt.Errorf("not found")},
			"git worktree": {},
		},
		interactiveErr: fmt.Errorf("setup failed"),
	}

	_, err := worktree.Create(r, cfg, worktree.CreateOpts{Name: "doomed"})

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
			"git fetch":    {},
			"git show-ref": {},
			"git worktree": {},
		},
		interactiveErr: fmt.Errorf("setup failed"),
	}

	_, err := worktree.Create(r, cfg, worktree.CreateOpts{Name: "preexisting"})

	require.Error(t, err)
	assert.True(t, r.hasCall("worktree remove --force"))
	assert.False(t, r.hasCall("branch -D"))
}

func TestDelete(t *testing.T) {
	wtBase := t.TempDir()
	r := &fakeRunner{responses: map[string]response{
		"git worktree": {},
	}}

	err := worktree.Delete(r, worktree.DeleteOpts{WorktreeBase: wtBase, Name: "my-feature"})

	require.NoError(t, err)
	assert.True(t, r.hasCall("worktree remove"))
	assert.True(t, r.hasCall("worktree prune"))
}

func TestDelete_RemoveFails(t *testing.T) {
	wtBase := t.TempDir()
	r := &fakeRunner{responses: map[string]response{
		"git worktree remove": {err: fmt.Errorf("locked")},
	}}

	err := worktree.Delete(r, worktree.DeleteOpts{WorktreeBase: wtBase, Name: "locked-wt"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "removing worktree")
}

func TestList_Error(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"git worktree list --porcelain": {err: fmt.Errorf("not a git repo")},
	}}

	_, err := worktree.List(r)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing worktrees")
}

func TestList(t *testing.T) {
	porcelain := "worktree /repo\nHEAD abc\nbranch refs/heads/main\n\nworktree /wt/feat\nHEAD def\nbranch refs/heads/feat\n\n"
	r := &fakeRunner{responses: map[string]response{
		"git worktree": {out: porcelain},
	}}

	wts, err := worktree.List(r)

	require.NoError(t, err)
	assert.Len(t, wts, 2)
	assert.Equal(t, "main", wts[0].Branch)
	assert.Equal(t, "feat", wts[1].Branch)
}
