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

func (f *fakeRunner) RunInteractive(_ string, _ ...string) error {
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

// newRepoDir creates a temp dir with a minimal .sarj.toml so config.Load succeeds.
func newRepoDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	wtBase := filepath.Join(dir, "wt")
	require.NoError(t, os.MkdirAll(wtBase, 0o750))

	cfg := fmt.Sprintf("worktree_base = %q\ndefault_branch = \"main\"\n", wtBase)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".sarj.toml"), []byte(cfg), 0o600))

	return dir
}
