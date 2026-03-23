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

// fakeRunner records calls and returns preconfigured responses.
// Matching tries the full command, then progressively shorter prefixes.
type fakeRunner struct {
	calls     []string
	responses map[string]response
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
	return nil
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

	cfg := &config.Config{
		WorktreeBase:  wtBase,
		DefaultBranch: "main",
	}

	r := &fakeRunner{
		responses: map[string]response{
			"git fetch":    {},
			"git show-ref": {err: fmt.Errorf("not found")},
			"git worktree": {},
		},
	}

	wtPath := filepath.Join(wtBase, "my-feature")

	wt, err := worktree.Create(r, cfg, worktree.CreateOpts{
		Name:      "my-feature",
		SkipSetup: true,
	})

	require.NoError(t, err)
	assert.Equal(t, "my-feature", wt.Branch)
	assert.Equal(t, wtPath, wt.Path)
}

func TestCreate_GeneratesName(t *testing.T) {
	wtBase := t.TempDir()

	cfg := &config.Config{
		WorktreeBase:  wtBase,
		DefaultBranch: "main",
	}

	r := &fakeRunner{
		responses: map[string]response{
			"git fetch":    {},
			"git show-ref": {err: fmt.Errorf("not found")},
			"git worktree": {},
		},
	}

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

func TestDelete(t *testing.T) {
	wtBase := t.TempDir()
	cfg := &config.Config{WorktreeBase: wtBase}

	r := &fakeRunner{
		responses: map[string]response{
			"git worktree": {},
			"git branch":   {},
		},
	}

	err := worktree.Delete(r, cfg, worktree.DeleteOpts{
		Name:         "my-feature",
		DeleteBranch: true,
	})

	require.NoError(t, err)
	assert.True(t, r.hasCall("worktree remove"))
	assert.True(t, r.hasCall("branch -D"))
}

func TestList(t *testing.T) {
	porcelain := "worktree /repo\nHEAD abc\nbranch refs/heads/main\n\nworktree /wt/feat\nHEAD def\nbranch refs/heads/feat\n\n"

	r := &fakeRunner{
		responses: map[string]response{
			"git worktree": {out: porcelain},
		},
	}

	wts, err := worktree.List(r)

	require.NoError(t, err)
	assert.Len(t, wts, 2)
	assert.Equal(t, "main", wts[0].Branch)
	assert.Equal(t, "feat", wts[1].Branch)
}
