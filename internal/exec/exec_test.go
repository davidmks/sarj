package exec_test

import (
	"testing"

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
			out, err := r.Run(tt.cmd, tt.args...)

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
	out, err := r.Run("pwd")

	require.NoError(t, err)
	// /tmp may resolve to /private/tmp on macOS
	assert.Contains(t, out, "tmp")
}
