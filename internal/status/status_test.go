package status_test

import (
	"context"
	"testing"
	"time"

	"github.com/davidmks/sarj/internal/exec"
	"github.com/davidmks/sarj/internal/status"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProbeAll(t *testing.T) {
	r := &exec.DefaultRunner{}
	items := []status.Item{
		{Branch: "feat-a", Path: "/wt/a"},
		{Branch: "feat-b", Path: "/wt/b"},
	}

	results := status.ProbeAll(t.Context(), r, `echo "$BRANCH"`, items, 5*time.Second)

	require.Len(t, results, 2)
	assert.Equal(t, "/wt/a", results[0].Path)
	assert.Equal(t, "feat-a", results[0].State)
	assert.NoError(t, results[0].Err)
	assert.Equal(t, "/wt/b", results[1].Path)
	assert.Equal(t, "feat-b", results[1].State)
}

func TestProbeAll_PathExposedAsEnv(t *testing.T) {
	r := &exec.DefaultRunner{}
	items := []status.Item{{Branch: "feat", Path: "/some/path"}}

	results := status.ProbeAll(t.Context(), r, `echo "$SARJ_WT_PATH"`, items, 5*time.Second)

	assert.Equal(t, "/some/path", results[0].State)
}

func TestProbeAll_NonZeroExit(t *testing.T) {
	r := &exec.DefaultRunner{}
	items := []status.Item{{Branch: "feat", Path: "/wt"}}

	results := status.ProbeAll(t.Context(), r, "false", items, 5*time.Second)

	assert.Equal(t, status.Unknown, results[0].State)
	require.Error(t, results[0].Err)
	assert.Contains(t, results[0].Err.Error(), "hook failed")
}

func TestProbeAll_EmptyOutput(t *testing.T) {
	r := &exec.DefaultRunner{}
	items := []status.Item{{Branch: "feat", Path: "/wt"}}

	results := status.ProbeAll(t.Context(), r, "true", items, 5*time.Second)

	assert.Equal(t, status.Unknown, results[0].State)
	assert.ErrorIs(t, results[0].Err, status.ErrEmptyOutput)
}

func TestProbeAll_Timeout(t *testing.T) {
	r := &exec.DefaultRunner{}
	items := []status.Item{{Branch: "feat", Path: "/wt"}}

	results := status.ProbeAll(t.Context(), r, "sleep 10", items, 100*time.Millisecond)

	assert.Equal(t, status.Unknown, results[0].State)
	require.Error(t, results[0].Err)
}

func TestProbeAll_TrimsWhitespace(t *testing.T) {
	r := &exec.DefaultRunner{}
	items := []status.Item{{Branch: "feat", Path: "/wt"}}

	results := status.ProbeAll(t.Context(), r, "printf 'merged\\n'", items, 5*time.Second)

	assert.Equal(t, "merged", results[0].State)
}

func TestProbeAll_DefaultTimeoutWhenZero(t *testing.T) {
	r := &exec.DefaultRunner{}
	items := []status.Item{{Branch: "feat", Path: "/wt"}}

	results := status.ProbeAll(t.Context(), r, "echo merged", items, 0)

	assert.Equal(t, "merged", results[0].State)
}

func TestProbeAll_ParentContextCanceled(t *testing.T) {
	r := &exec.DefaultRunner{}
	items := []status.Item{{Branch: "feat", Path: "/wt"}}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	results := status.ProbeAll(ctx, r, "echo merged", items, 5*time.Second)
	assert.Equal(t, status.Unknown, results[0].State)
}

func TestProbeAll_ManyItemsRespectMaxParallel(t *testing.T) {
	r := &exec.DefaultRunner{}
	const n = status.MaxParallel * 3
	items := make([]status.Item, n)
	for i := range items {
		items[i] = status.Item{Branch: "feat", Path: "/wt"}
	}

	results := status.ProbeAll(t.Context(), r, "echo merged", items, 5*time.Second)

	require.Len(t, results, n)
	for _, res := range results {
		assert.Equal(t, "merged", res.State)
	}
}
