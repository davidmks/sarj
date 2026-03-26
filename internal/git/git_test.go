package git_test

import (
	"fmt"
	"testing"

	"github.com/davidmks/sarj/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRunner returns preconfigured output for testing.
type fakeRunner struct {
	out string
	err error
}

func (f *fakeRunner) Run(_ string, _ ...string) (string, error) {
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
