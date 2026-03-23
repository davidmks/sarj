package tmux_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/davidmks/sarj/internal/config"
	"github.com/davidmks/sarj/internal/tmux"
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

func (f *fakeRunner) RunInteractive(name string, args ...string) error {
	call := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, call)

	if resp, ok := f.responses[call]; ok {
		return resp.err
	}
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

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "dots replaced", in: "feat.v2", want: "feat-v2"},
		{name: "colons replaced", in: "feat:v2", want: "feat-v2"},
		{name: "both replaced", in: "my.feat:v2", want: "my-feat-v2"},
		{name: "no change needed", in: "my-feature", want: "my-feature"},
		{name: "slashes preserved", in: "feat/v2", want: "feat/v2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tmux.SanitizeName(tt.in))
		})
	}
}

func TestIsInstalled(t *testing.T) {
	t.Run("installed", func(t *testing.T) {
		r := &fakeRunner{responses: map[string]response{
			"tmux -V": {out: "tmux 3.4"},
		}}
		assert.True(t, tmux.IsInstalled(r))
	})

	t.Run("not installed", func(t *testing.T) {
		r := &fakeRunner{responses: map[string]response{
			"tmux -V": {err: fmt.Errorf("not found")},
		}}
		assert.False(t, tmux.IsInstalled(r))
	})
}

func TestIsInsideSession(t *testing.T) {
	t.Run("inside", func(t *testing.T) {
		t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")
		assert.True(t, tmux.IsInsideSession())
	})

	t.Run("outside", func(t *testing.T) {
		t.Setenv("TMUX", "")
		assert.False(t, tmux.IsInsideSession())
	})
}

func TestSessionExists(t *testing.T) {
	t.Run("exists", func(t *testing.T) {
		r := &fakeRunner{responses: map[string]response{
			"tmux has-session -t my-session": {},
		}}
		assert.True(t, tmux.SessionExists(r, "my-session"))
	})

	t.Run("not exists", func(t *testing.T) {
		r := &fakeRunner{responses: map[string]response{
			"tmux has-session -t my-session": {err: fmt.Errorf("no session")},
		}}
		assert.False(t, tmux.SessionExists(r, "my-session"))
	})
}

func TestCreateSession_SingleWindow(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{}}

	windows := []config.WindowConfig{
		{Name: "terminal"},
	}

	err := tmux.CreateSession(r, "my-session", "/work/repo", windows)
	require.NoError(t, err)

	assert.True(t, r.hasCall("new-session -d -s my-session -c /work/repo -n terminal"))
	assert.True(t, r.hasCall("select-window -t my-session:terminal"))
}

func TestCreateSession_MultipleWindows(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{}}

	windows := []config.WindowConfig{
		{Name: "terminal"},
		{Name: "editor", Command: "nvim ."},
		{Name: "claude", Command: "claude"},
	}

	err := tmux.CreateSession(r, "my-session", "/work/repo", windows)
	require.NoError(t, err)

	assert.True(t, r.hasCall("new-session -d -s my-session"))
	assert.True(t, r.hasCall("new-window -t my-session -n editor"))
	assert.True(t, r.hasCall("send-keys -t my-session:editor nvim . Enter"))
	assert.True(t, r.hasCall("new-window -t my-session -n claude"))
	assert.True(t, r.hasCall("send-keys -t my-session:claude claude Enter"))
}

func TestCreateSession_WithPanes(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{}}

	windows := []config.WindowConfig{
		{
			Name: "dev",
			Panes: []config.PaneConfig{
				{Command: "nvim ."},
				{Command: "make watch", Split: "horizontal", Size: 30},
			},
		},
	}

	err := tmux.CreateSession(r, "my-session", "/work/repo", windows)
	require.NoError(t, err)

	assert.True(t, r.hasCall("send-keys -t my-session:dev nvim . Enter"))
	assert.True(t, r.hasCall("split-window -h -t my-session:dev -c /work/repo -p 30"))
	assert.True(t, r.hasCall("send-keys -t my-session:dev make watch Enter"))
}

func TestCreateSession_SanitizesName(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{}}

	err := tmux.CreateSession(r, "feat.v2", "/work", []config.WindowConfig{{Name: "terminal"}})
	require.NoError(t, err)

	assert.True(t, r.hasCall("-s feat-v2"))
}

func TestCreateSession_DefaultWindow(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{}}

	err := tmux.CreateSession(r, "test", "/work", nil)
	require.NoError(t, err)

	assert.True(t, r.hasCall("-n terminal"))
}

func TestCreateSession_NewSessionFails(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"tmux new-session": {err: fmt.Errorf("duplicate session")},
	}}

	err := tmux.CreateSession(r, "my-session", "/work", []config.WindowConfig{{Name: "terminal"}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating tmux session")
}

func TestCreateSession_NewWindowFails(t *testing.T) {
	r := &fakeRunner{responses: map[string]response{
		"tmux new-window": {err: fmt.Errorf("window error")},
	}}

	windows := []config.WindowConfig{
		{Name: "terminal"},
		{Name: "editor", Command: "nvim ."},
	}

	err := tmux.CreateSession(r, "my-session", "/work", windows)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating tmux window")
}

func TestKillSession(t *testing.T) {
	t.Run("exists", func(t *testing.T) {
		r := &fakeRunner{responses: map[string]response{
			"tmux has-session -t my-session": {},
		}}

		err := tmux.KillSession(r, "my-session")
		require.NoError(t, err)
		assert.True(t, r.hasCall("kill-session -t my-session"))
	})

	t.Run("does not exist", func(t *testing.T) {
		r := &fakeRunner{responses: map[string]response{
			"tmux has-session -t my-session": {err: fmt.Errorf("no session")},
		}}

		err := tmux.KillSession(r, "my-session")
		require.NoError(t, err)
		assert.False(t, r.hasCall("kill-session"))
	})

	t.Run("sanitizes name", func(t *testing.T) {
		r := &fakeRunner{responses: map[string]response{
			"tmux has-session -t feat-v2": {},
		}}

		err := tmux.KillSession(r, "feat.v2")
		require.NoError(t, err)
		assert.True(t, r.hasCall("kill-session -t feat-v2"))
	})
}

func TestConnect(t *testing.T) {
	t.Run("outside tmux attaches", func(t *testing.T) {
		t.Setenv("TMUX", "")
		r := &fakeRunner{responses: map[string]response{}}

		err := tmux.Connect(r, "my-session")
		require.NoError(t, err)
		assert.True(t, r.hasCall("attach-session -t my-session"))
	})

	t.Run("inside tmux switches", func(t *testing.T) {
		t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")
		r := &fakeRunner{responses: map[string]response{}}

		err := tmux.Connect(r, "my-session")
		require.NoError(t, err)
		assert.True(t, r.hasCall("switch-client -t my-session"))
	})

	t.Run("sanitizes name", func(t *testing.T) {
		t.Setenv("TMUX", "")
		r := &fakeRunner{responses: map[string]response{}}

		err := tmux.Connect(r, "feat.v2")
		require.NoError(t, err)
		assert.True(t, r.hasCall("attach-session -t feat-v2"))
	})
}

func TestListSessions(t *testing.T) {
	t.Run("returns sessions", func(t *testing.T) {
		r := &fakeRunner{responses: map[string]response{
			"tmux list-sessions -F #{session_name}": {out: "foo\nbar\nbaz"},
		}}

		sessions, err := tmux.ListSessions(r)
		require.NoError(t, err)
		assert.Equal(t, []string{"foo", "bar", "baz"}, sessions)
	})

	t.Run("no server running", func(t *testing.T) {
		r := &fakeRunner{responses: map[string]response{
			"tmux list-sessions -F #{session_name}": {err: fmt.Errorf("no server running")},
		}}

		sessions, err := tmux.ListSessions(r)
		require.NoError(t, err)
		assert.Nil(t, sessions)
	})

	t.Run("empty", func(t *testing.T) {
		r := &fakeRunner{responses: map[string]response{
			"tmux list-sessions -F #{session_name}": {out: ""},
		}}

		sessions, err := tmux.ListSessions(r)
		require.NoError(t, err)
		assert.Nil(t, sessions)
	})
}

func TestBuildCommand(t *testing.T) {
	tests := []struct {
		name    string
		envFile string
		env     map[string]string
		command string
		want    string
	}{
		{
			name:    "command only",
			command: "nvim .",
			want:    "nvim .",
		},
		{
			name:    "env file only",
			envFile: ".env.test",
			want:    "set -a && source .env.test && set +a",
		},
		{
			name:    "env vars only",
			env:     map[string]string{"FOO": "bar"},
			want:    "export FOO=bar",
		},
		{
			name:    "all combined",
			envFile: ".env.test",
			env:     map[string]string{"UV_ENV_FILE": ".env"},
			command: "nvim .",
			want:    "set -a && source .env.test && set +a && export UV_ENV_FILE=.env && nvim .",
		},
		{
			name:    "env vars sorted",
			env:     map[string]string{"Z_VAR": "z", "A_VAR": "a"},
			command: "bash",
			want:    "export A_VAR=a Z_VAR=z && bash",
		},
		{
			name: "empty",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tmux.BuildCommand(tt.envFile, tt.env, tt.command)
			assert.Equal(t, tt.want, got)
		})
	}
}
