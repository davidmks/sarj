package worktree

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePorcelain(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   []Worktree
	}{
		{
			name: "single main worktree",
			input: "worktree /home/user/repo\nHEAD abc123\nbranch refs/heads/main\n\n",
			want: []Worktree{
				{Path: "/home/user/repo", Branch: "main", HEAD: "abc123"},
			},
		},
		{
			name: "multiple worktrees",
			input: "worktree /home/user/repo\nHEAD abc123\nbranch refs/heads/main\n\nworktree /home/user/wt/feature\nHEAD def456\nbranch refs/heads/feature\n\n",
			want: []Worktree{
				{Path: "/home/user/repo", Branch: "main", HEAD: "abc123"},
				{Path: "/home/user/wt/feature", Branch: "feature", HEAD: "def456"},
			},
		},
		{
			name: "bare repo",
			input: "worktree /home/user/repo.git\nHEAD abc123\nbare\n\n",
			want: []Worktree{
				{Path: "/home/user/repo.git", HEAD: "abc123", Bare: true},
			},
		},
		{
			name:  "no trailing newline",
			input: "worktree /home/user/repo\nHEAD abc123\nbranch refs/heads/main",
			want: []Worktree{
				{Path: "/home/user/repo", Branch: "main", HEAD: "abc123"},
			},
		},
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePorcelain(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
