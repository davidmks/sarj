package worktree_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/davidmks/sarj/internal/config"
	"github.com/davidmks/sarj/internal/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRunner records calls and returns preconfigured responses.
type fakeRunner struct {
	calls []string
	// responses maps a command key ("git worktree add") to its output/error
	responses map[string]response
}

type response struct {
	out string
	err error
}

func (f *fakeRunner) Run(name string, args ...string) (string, error) {
	key := name
	if len(args) > 0 {
		key = name + " " + args[0]
	}
	f.calls = append(f.calls, fmt.Sprintf("%s %s", name, joinArgs(args)))

	if resp, ok := f.responses[key]; ok {
		return resp.out, resp.err
	}
	return "", nil
}

func (f *fakeRunner) RunInteractive(_ string, _ ...string) error {
	return nil
}

func joinArgs(args []string) string {
	s := ""
	for i, a := range args {
		if i > 0 {
			s += " "
		}
		s += a
	}
	return s
}

func TestCreate_NewBranch(t *testing.T) {
	wtBase := t.TempDir()

	cfg := &config.Config{
		WorktreeBase:  wtBase,
		DefaultBranch: "main",
	}

	r := &fakeRunner{
		responses: map[string]response{
			// fetch succeeds
			"git fetch": {},
			// branch doesn't exist
			"git show-ref": {err: fmt.Errorf("not found")},
			// worktree add succeeds — we need to create the dir since Create checks os.Stat
			"git worktree": {},
		},
	}

	// Pre-create the worktree dir to simulate git worktree add
	wtPath := filepath.Join(wtBase, "my-feature")
	require.NoError(t, os.MkdirAll(wtPath, 0o755))

	// Remove it so the existence check passes, then re-create via a custom response
	require.NoError(t, os.RemoveAll(wtPath))

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
	require.NoError(t, os.MkdirAll(filepath.Join(wtBase, "existing"), 0o755))

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
	assert.Contains(t, r.calls[0], "worktree remove")
	assert.Contains(t, r.calls[1], "branch -D")
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
