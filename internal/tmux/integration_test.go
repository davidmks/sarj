//go:build integration

package tmux_test

import (
	osexec "os/exec"
	"testing"

	"github.com/davidmks/sarj/internal/config"
	"github.com/davidmks/sarj/internal/exec"
	"github.com/davidmks/sarj/internal/tmux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireTmux(t *testing.T) {
	t.Helper()
	if _, err := osexec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
}

func TestIntegration_CreateAndKillSession(t *testing.T) {
	requireTmux(t)
	r := &exec.DefaultRunner{}

	sessionName := "sarj-test-basic"
	t.Cleanup(func() {
		tmux.KillSession(r, sessionName)
	})

	windows := []config.WindowConfig{
		{Name: "terminal"},
		{Name: "editor", Command: "echo hello"},
	}

	err := tmux.CreateSession(r, sessionName, t.TempDir(), windows)
	require.NoError(t, err)
	assert.True(t, tmux.SessionExists(r, sessionName))

	out, err := r.Run("tmux", "list-windows", "-t", sessionName, "-F", "#{window_name}")
	require.NoError(t, err)
	assert.Contains(t, out, "terminal")
	assert.Contains(t, out, "editor")

	err = tmux.KillSession(r, sessionName)
	require.NoError(t, err)
	assert.False(t, tmux.SessionExists(r, sessionName))
}

func TestIntegration_SessionWithPanes(t *testing.T) {
	requireTmux(t)
	r := &exec.DefaultRunner{}

	sessionName := "sarj-test-panes"
	t.Cleanup(func() {
		tmux.KillSession(r, sessionName)
	})

	windows := []config.WindowConfig{
		{
			Name: "dev",
			Panes: []config.PaneConfig{
				{Command: "echo pane1"},
				{Command: "echo pane2", Split: "horizontal"},
			},
		},
	}

	err := tmux.CreateSession(r, sessionName, t.TempDir(), windows)
	require.NoError(t, err)

	out, err := r.Run("tmux", "list-panes", "-t", sessionName+":dev", "-F", "#{pane_index}")
	require.NoError(t, err)
	assert.Contains(t, out, "0")
	assert.Contains(t, out, "1")
}

func TestIntegration_Focus(t *testing.T) {
	requireTmux(t)
	r := &exec.DefaultRunner{}

	sessionName := "sarj-test-focus"
	t.Cleanup(func() {
		tmux.KillSession(r, sessionName)
	})

	windows := []config.WindowConfig{
		{
			Name: "dev",
			Panes: []config.PaneConfig{
				{Command: "echo pane0", Focus: true},
				{Command: "echo pane1", Split: "horizontal"},
			},
		},
		{Name: "shell", Focus: true},
	}

	err := tmux.CreateSession(r, sessionName, t.TempDir(), windows)
	require.NoError(t, err)

	activeWindow, err := r.Run("tmux", "display-message", "-t", sessionName, "-p", "#{window_name}")
	require.NoError(t, err)
	assert.Equal(t, "shell", activeWindow)

	activePane, err := r.Run("tmux", "display-message", "-t", sessionName+":dev", "-p", "#{pane_index}")
	require.NoError(t, err)
	assert.Equal(t, "0", activePane)
}

func TestIntegration_ListSessions(t *testing.T) {
	requireTmux(t)
	r := &exec.DefaultRunner{}

	sessionName := "sarj-test-list"
	t.Cleanup(func() {
		tmux.KillSession(r, sessionName)
	})

	err := tmux.CreateSession(r, sessionName, t.TempDir(), nil)
	require.NoError(t, err)

	sessions, err := tmux.ListSessions(r)
	require.NoError(t, err)
	assert.Contains(t, sessions, sessionName)
}

func TestIntegration_SessionWithPaneSizes(t *testing.T) {
	requireTmux(t)
	r := &exec.DefaultRunner{}

	sessionName := "sarj-test-pane-sizes"
	t.Cleanup(func() {
		tmux.KillSession(r, sessionName)
	})

	windows := []config.WindowConfig{
		{
			Name: "dev",
			Panes: []config.PaneConfig{
				{Command: "echo pane1"},
				{Command: "echo pane2", Split: "horizontal", Size: 30},
			},
		},
	}

	err := tmux.CreateSession(r, sessionName, t.TempDir(), windows)
	require.NoError(t, err)

	out, err := r.Run("tmux", "list-panes", "-t", sessionName+":dev", "-F", "#{pane_index}")
	require.NoError(t, err)
	assert.Contains(t, out, "0")
	assert.Contains(t, out, "1")
}

func TestIntegration_KillNonexistentSession(t *testing.T) {
	requireTmux(t)
	r := &exec.DefaultRunner{}

	err := tmux.KillSession(r, "sarj-definitely-not-a-session")
	require.NoError(t, err)
}

func TestIntegration_SanitizedSessionName(t *testing.T) {
	requireTmux(t)
	r := &exec.DefaultRunner{}

	t.Cleanup(func() {
		tmux.KillSession(r, "feat.v2")
	})

	err := tmux.CreateSession(r, "feat.v2", t.TempDir(), nil)
	require.NoError(t, err)

	assert.True(t, tmux.SessionExists(r, "feat-v2"))
	assert.True(t, tmux.SessionExists(r, "feat.v2"), "unsanitized name should also resolve")
}
