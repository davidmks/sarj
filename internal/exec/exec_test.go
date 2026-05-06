package exec_test

import (
	"context"
	"testing"
	"time"

	"github.com/davidmks/sarj/internal/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultRunner_Run(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		args    []string
		wantOut string
		wantErr bool
	}{
		{
			name:    "simple echo",
			cmd:     "echo",
			args:    []string{"hello"},
			wantOut: "hello",
		},
		{
			name:    "trims whitespace",
			cmd:     "echo",
			args:    []string{"-n", "  trimmed  "},
			wantOut: "trimmed",
		},
		{
			name:    "command not found",
			cmd:     "definitely-not-a-real-command",
			wantErr: true,
		},
		{
			name:    "nonzero exit",
			cmd:     "false",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &exec.DefaultRunner{}
			out, err := r.Run(t.Context(), tt.cmd, tt.args...)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantOut, out)
		})
	}
}

func TestDefaultRunner_Run_Dir(t *testing.T) {
	r := &exec.DefaultRunner{Dir: "/tmp"}
	out, err := r.Run(t.Context(), "pwd")

	require.NoError(t, err)
	// /tmp may resolve to /private/tmp on macOS
	assert.Contains(t, out, "tmp")
}

func TestDefaultRunner_RunWithEnv(t *testing.T) {
	r := &exec.DefaultRunner{}
	out, err := r.RunWithEnv(t.Context(), []string{"FOO=bar"}, "sh", "-c", `echo "$FOO"`)

	require.NoError(t, err)
	assert.Equal(t, "bar", out)
}

// TestDefaultRunner_RunWithEnv_StdoutOnly guards the contract that the
// status hook returns stdout, not combined output: stderr from the hook
// must never end up concatenated into the state token.
func TestDefaultRunner_RunWithEnv_StdoutOnly(t *testing.T) {
	r := &exec.DefaultRunner{}
	out, err := r.RunWithEnv(t.Context(), nil, "sh", "-c",
		`echo "warning: noise" >&2; echo merged`)

	require.NoError(t, err)
	assert.Equal(t, "merged", out)
}

// TestDefaultRunner_RunWithEnv_StderrInError verifies stderr surfaces via
// the error when the command fails, so users can debug a broken hook.
func TestDefaultRunner_RunWithEnv_StderrInError(t *testing.T) {
	r := &exec.DefaultRunner{}
	_, err := r.RunWithEnv(t.Context(), nil, "sh", "-c",
		`echo "boom" >&2; exit 1`)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

// TestDefaultRunner_RunWithEnv_CompoundTimeoutWallClock guards against the
// orphaned-grandchild hang: `sh -c "A; B"` forks A, so killing sh leaves A
// holding the output pipes. WaitDelay must force-close them so Wait returns
// near the deadline rather than the natural runtime of A.
func TestDefaultRunner_RunWithEnv_CompoundTimeoutWallClock(t *testing.T) {
	r := &exec.DefaultRunner{}
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := r.RunWithEnv(ctx, nil, "sh", "-c", "sleep 5; echo never")
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Less(t, elapsed, 2*time.Second,
		"compound-command timeout must wrap up promptly via WaitDelay (got %s)", elapsed)
}
