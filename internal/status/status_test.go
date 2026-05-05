package status_test

import (
	"context"
	"testing"
	"time"

	"github.com/davidmks/sarj/internal/exec"
	"github.com/davidmks/sarj/internal/status"
	"github.com/stretchr/testify/assert"
)

func TestBuild(t *testing.T) {
	tests := []struct {
		name     string
		tmpl     string
		branch   string
		path     string
		expected string
	}{
		{
			name:     "branch only",
			tmpl:     "gh pr view {{.Branch}}",
			branch:   "feat-x",
			path:     "/wt/feat-x",
			expected: "gh pr view feat-x",
		},
		{
			name:     "branch and path",
			tmpl:     "check {{.Branch}} at {{.Path}}",
			branch:   "feat-x",
			path:     "/wt/feat-x",
			expected: "check feat-x at /wt/feat-x",
		},
		{
			name:     "no placeholders",
			tmpl:     "echo merged",
			branch:   "feat-x",
			path:     "/wt/feat-x",
			expected: "echo merged",
		},
		{
			name:     "repeated placeholder",
			tmpl:     "{{.Branch}} {{.Branch}}",
			branch:   "feat",
			path:     "",
			expected: "feat feat",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, status.Build(tt.tmpl, tt.branch, tt.path))
		})
	}
}

func TestProbeAll(t *testing.T) {
	r := &exec.DefaultRunner{}
	items := []status.Item{
		{Branch: "feat-a", Path: "/wt/a"},
		{Branch: "feat-b", Path: "/wt/b"},
	}

	results := status.ProbeAll(context.Background(), r, "echo {{.Branch}}", items, 5*time.Second)

	assert.Equal(t, []status.Result{
		{Path: "/wt/a", State: "feat-a"},
		{Path: "/wt/b", State: "feat-b"},
	}, results)
}

func TestProbeAll_NonZeroExit(t *testing.T) {
	r := &exec.DefaultRunner{}
	items := []status.Item{{Branch: "feat", Path: "/wt"}}

	results := status.ProbeAll(context.Background(), r, "false", items, 5*time.Second)

	assert.Equal(t, status.Unknown, results[0].State)
}

func TestProbeAll_EmptyOutput(t *testing.T) {
	r := &exec.DefaultRunner{}
	items := []status.Item{{Branch: "feat", Path: "/wt"}}

	results := status.ProbeAll(context.Background(), r, "true", items, 5*time.Second)

	assert.Equal(t, status.Unknown, results[0].State)
}

func TestProbeAll_Timeout(t *testing.T) {
	r := &exec.DefaultRunner{}
	items := []status.Item{{Branch: "feat", Path: "/wt"}}

	start := time.Now()
	results := status.ProbeAll(context.Background(), r, "sleep 10", items, 100*time.Millisecond)
	elapsed := time.Since(start)

	assert.Equal(t, status.Unknown, results[0].State)
	assert.Less(t, elapsed, 5*time.Second, "timeout should kick in well before sleep finishes")
}

func TestProbeAll_TrimsWhitespace(t *testing.T) {
	r := &exec.DefaultRunner{}
	items := []status.Item{{Branch: "feat", Path: "/wt"}}

	results := status.ProbeAll(context.Background(), r, "printf 'merged\\n'", items, 5*time.Second)

	assert.Equal(t, "merged", results[0].State)
}

func TestProbeAll_DefaultTimeoutWhenZero(t *testing.T) {
	r := &exec.DefaultRunner{}
	items := []status.Item{{Branch: "feat", Path: "/wt"}}

	results := status.ProbeAll(context.Background(), r, "echo merged", items, 0)

	assert.Equal(t, "merged", results[0].State)
}

func TestProbeAll_ParentContextCanceled(t *testing.T) {
	r := &exec.DefaultRunner{}
	items := []status.Item{{Branch: "feat", Path: "/wt"}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	results := status.ProbeAll(ctx, r, "echo merged", items, 5*time.Second)
	assert.Equal(t, status.Unknown, results[0].State)
}
