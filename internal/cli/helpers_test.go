package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeRunner struct {
	calls     []string
	responses map[string]response
}

type response struct {
	out string
	err error
}

// Matching tries the full command, then progressively shorter prefixes.
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

func (f *fakeRunner) RunInteractive(name string, args ...string) error {
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	return nil
}

func (f *fakeRunner) hasCall(substr string) bool {
	return f.indexOfCall(substr) >= 0
}

func (f *fakeRunner) indexOfCall(substr string) int {
	for i, c := range f.calls {
		if strings.Contains(c, substr) {
			return i
		}
	}
	return -1
}

// isolateConfig prevents tests from loading the user's real global config.
func isolateConfig(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

// saveCwd saves the current working directory and restores it when the test finishes.
// Needed because the delete command calls os.Chdir which is process-global.
func saveCwd(t *testing.T) {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(dir) })
}

// fakeWorktreeDir creates a directory at the given path so Delete's os.Stat
// finds it during tests.
func fakeWorktreeDir(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(path, 0o750))
}

// newRepoDir creates a temp dir with minimal config so config.Load succeeds.
// worktree_base is set via local config because project-level merge does not
// override it from the global default.
func newRepoDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	wtBase := filepath.Join(dir, "wt")
	require.NoError(t, os.MkdirAll(wtBase, 0o750))

	proj := "default_branch = \"main\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".sarj.toml"), []byte(proj), 0o600))

	local := fmt.Sprintf("worktree_base = %q\n", wtBase)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".sarj.local.toml"), []byte(local), 0o600))

	return dir
}
