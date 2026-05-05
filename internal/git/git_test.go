package git_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/davidmks/sarj/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRunner returns preconfigured output for testing. Each Run records the
// arguments it received so tests that care about command shape can assert on
// lastArgs; tests that don't can ignore the field.
type fakeRunner struct {
	out      string
	err      error
	lastCmd  string
	lastArgs []string
}

func (f *fakeRunner) Run(name string, args ...string) (string, error) {
	f.lastCmd = name
	f.lastArgs = args
	return f.out, f.err
}

func (f *fakeRunner) RunInteractive(_ string, _ ...string) error {
	return f.err
}

func TestRepoRoot(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		err     error
		want    string
		wantErr bool
	}{
		{
			name: "returns repo root",
			out:  "/home/user/myrepo",
			want: "/home/user/myrepo",
		},
		{
			name:    "not a git repo",
			err:     fmt.Errorf("not a git repository"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{out: tt.out, err: tt.err}
			got, err := git.RepoRoot(r)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMainWorktree(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		err     error
		want    string
		wantErr bool
	}{
		{
			name: "parses porcelain output",
			out:  "worktree /home/user/myrepo\nHEAD abc123\nbranch refs/heads/main\n",
			want: "/home/user/myrepo",
		},
		{
			name:    "git error",
			err:     fmt.Errorf("failed"),
			wantErr: true,
		},
		{
			name:    "empty output",
			out:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{out: tt.out, err: tt.err}
			got, err := git.MainWorktree(r)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDefaultBranch(t *testing.T) {
	tests := []struct {
		name string
		out  string
		err  error
		want string
	}{
		{
			name: "detects main",
			out:  "refs/remotes/origin/main",
			want: "main",
		},
		{
			name: "detects trunk",
			out:  "refs/remotes/origin/trunk",
			want: "trunk",
		},
		{
			name: "falls back on error",
			err:  fmt.Errorf("no remote"),
			want: "main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{out: tt.out, err: tt.err}
			got := git.DefaultBranch(r)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFetch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		r := &fakeRunner{}
		err := git.Fetch(r, "origin")
		require.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		r := &fakeRunner{err: fmt.Errorf("network error")}
		err := git.Fetch(r, "origin")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "fetching origin")
	})
}

func TestBranchExists(t *testing.T) {
	t.Run("exists", func(t *testing.T) {
		r := &fakeRunner{}
		assert.True(t, git.BranchExists(r, "main"))
	})

	t.Run("does not exist", func(t *testing.T) {
		r := &fakeRunner{err: fmt.Errorf("not found")}
		assert.False(t, git.BranchExists(r, "nope"))
	})
}

func TestRemoteRefExists(t *testing.T) {
	t.Run("exists", func(t *testing.T) {
		r := &fakeRunner{}
		assert.True(t, git.RemoteRefExists(r, "origin/main"))
	})

	t.Run("does not exist", func(t *testing.T) {
		r := &fakeRunner{err: fmt.Errorf("not found")}
		assert.False(t, git.RemoteRefExists(r, "origin/nope"))
	})
}

func TestDirty(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want bool
	}{
		{name: "clean", out: "", want: false},
		{name: "modified", out: " M file.go\n", want: true},
		{name: "untracked", out: "?? new.go\n", want: true},
		{name: "whitespace only treated as clean", out: "\n", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{out: tt.out}
			got, err := git.Dirty(r, "/some/path")
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, []string{"-C", "/some/path", "status", "--porcelain"}, r.lastArgs)
		})
	}

	t.Run("propagates error", func(t *testing.T) {
		r := &fakeRunner{err: fmt.Errorf("not a repo")}
		_, err := git.Dirty(r, "/x")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "checking dirty state")
	})
}

func TestHeadInfo(t *testing.T) {
	t.Run("parses date and subject", func(t *testing.T) {
		r := &fakeRunner{out: "2026-05-04T10:23:00+02:00\nfix: handle empty input\n"}
		subject, date, err := git.HeadInfo(r, "/wt/foo")
		require.NoError(t, err)
		assert.Equal(t, "fix: handle empty input", subject)
		want, _ := time.Parse(time.RFC3339, "2026-05-04T10:23:00+02:00")
		assert.True(t, date.Equal(want))
		assert.Equal(t, []string{"-C", "/wt/foo", "log", "-1", "--format=%cI%n%s"}, r.lastArgs)
	})

	t.Run("subject with newlines is split on first only", func(t *testing.T) {
		r := &fakeRunner{out: "2026-05-04T10:23:00Z\nline one\nline two\n"}
		subject, _, err := git.HeadInfo(r, "/wt/foo")
		require.NoError(t, err)
		assert.Equal(t, "line one\nline two", subject)
	})

	t.Run("empty output errors", func(t *testing.T) {
		r := &fakeRunner{}
		_, _, err := git.HeadInfo(r, "/wt/foo")
		require.Error(t, err)
	})

	t.Run("invalid date errors", func(t *testing.T) {
		r := &fakeRunner{out: "not-a-date\nsubject\n"}
		_, _, err := git.HeadInfo(r, "/wt/foo")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parsing date")
	})

	t.Run("propagates git error", func(t *testing.T) {
		r := &fakeRunner{err: fmt.Errorf("does not have any commits yet")}
		_, _, err := git.HeadInfo(r, "/wt/foo")
		require.Error(t, err)
	})
}

func TestUpstream(t *testing.T) {
	tests := []struct {
		name       string
		out        string
		err        error
		wantRemote string
		wantBranch string
		wantErr    bool
	}{
		{name: "origin/main", out: "origin/main\n", wantRemote: "origin", wantBranch: "main"},
		{name: "branch with slash", out: "origin/feat/sub\n", wantRemote: "origin", wantBranch: "feat/sub"},
		{name: "no upstream configured", err: fmt.Errorf("no upstream configured"), wantErr: true},
		{name: "empty output", out: "\n", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{out: tt.out, err: tt.err}
			remote, branch, err := git.Upstream(r, "/wt/foo")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantRemote, remote)
			assert.Equal(t, tt.wantBranch, branch)
			assert.True(t, strings.Contains(strings.Join(r.lastArgs, " "), "@{u}"))
		})
	}
}

func TestAheadBehind(t *testing.T) {
	tests := []struct {
		name       string
		out        string
		err        error
		wantAhead  int
		wantBehind int
		wantErr    bool
	}{
		{name: "ahead 3, behind 0", out: "3\t0\n", wantAhead: 3, wantBehind: 0},
		{name: "ahead 0, behind 12", out: "0\t12\n", wantAhead: 0, wantBehind: 12},
		{name: "in sync", out: "0\t0\n", wantAhead: 0, wantBehind: 0},
		{name: "git error", err: fmt.Errorf("bad ref"), wantErr: true},
		{name: "malformed output", out: "garbage\n", wantErr: true},
		{name: "non-numeric counts", out: "x\ty\n", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{out: tt.out, err: tt.err}
			ahead, behind, err := git.AheadBehind(r, "/wt/foo", "origin/foo")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantAhead, ahead)
			assert.Equal(t, tt.wantBehind, behind)
			assert.Contains(t, r.lastArgs, "HEAD...origin/foo")
		})
	}
}

func TestCommitsBehind(t *testing.T) {
	tests := []struct {
		name string
		out  string
		err  error
		want int
	}{
		{
			name: "behind by 3",
			out:  "3",
			want: 3,
		},
		{
			name: "not behind",
			out:  "0",
			want: 0,
		},
		{
			name: "error returns zero",
			err:  fmt.Errorf("unknown revision"),
			want: 0,
		},
		{
			name: "invalid output returns zero",
			out:  "notanumber",
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{out: tt.out, err: tt.err}
			got := git.CommitsBehind(r, "feature", "origin/feature")
			assert.Equal(t, tt.want, got)
		})
	}
}
