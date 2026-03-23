package worktree_test

import (
	"strings"
	"testing"

	"github.com/davidmks/sarj/internal/worktree"
	"github.com/stretchr/testify/assert"
)

func TestGenerateName(t *testing.T) {
	t.Run("format is adjective-verbing-noun", func(t *testing.T) {
		name := worktree.GenerateName()
		parts := strings.Split(name, "-")
		assert.Len(t, parts, 3)
		assert.True(t, strings.HasSuffix(parts[1], "ing"), "middle word should end in -ing, got %q", parts[1])
	})

	t.Run("generates different names", func(t *testing.T) {
		seen := make(map[string]bool)
		for range 20 {
			seen[worktree.GenerateName()] = true
		}
		assert.Greater(t, len(seen), 1, "expected varied names across 20 generations")
	})
}
