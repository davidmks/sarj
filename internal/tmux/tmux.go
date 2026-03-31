// Package tmux manages tmux sessions, windows, and panes via the tmux CLI.
package tmux

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/davidmks/sarj/internal/config"
	"github.com/davidmks/sarj/internal/exec"
)

var nameReplacer = strings.NewReplacer(".", "-", ":", "-", "/", "-")

// SanitizeName replaces characters that are problematic in tmux session names
// (., :, and /) with -.
func SanitizeName(name string) string {
	return nameReplacer.Replace(name)
}

// IsInstalled checks whether the tmux binary works (not just present in PATH).
func IsInstalled(r exec.Runner) bool {
	_, err := r.Run("tmux", "-V")
	return err == nil
}

// IsInsideSession returns true when running inside an existing tmux session.
func IsInsideSession() bool {
	return os.Getenv("TMUX") != ""
}

// SessionExists checks whether a tmux session with the given name exists.
func SessionExists(r exec.Runner, name string) bool {
	_, err := r.Run("tmux", "has-session", "-t", SanitizeName(name))
	return err == nil
}

// CreateSession creates a new tmux session with the configured windows and panes.
// The session is created in detached mode; call Connect to connect.
// The args string is substituted into any command containing {{.Args}}.
func CreateSession(r exec.Runner, name, path string, windows []config.WindowConfig, cmdArgs string) error {
	name = SanitizeName(name)

	if len(windows) == 0 {
		windows = []config.WindowConfig{{Name: "terminal"}}
	}

	// Query pane-base-index up front so no tmux round-trip sits between
	// the last send-keys and select-window.
	baseIdx := paneBaseIndex(r)

	first := windows[0]
	args := []string{"new-session", "-d", "-s", name, "-c", path, "-n", first.Name}
	if _, err := r.Run("tmux", args...); err != nil {
		return fmt.Errorf("creating tmux session %s: %w", name, err)
	}

	if err := sendWindowCommand(r, name, first, cmdArgs); err != nil {
		return err
	}
	if err := createPanes(r, name, first, path, cmdArgs); err != nil {
		return err
	}

	for _, w := range windows[1:] {
		wArgs := []string{"new-window", "-t", name, "-n", w.Name, "-c", path}
		if _, err := r.Run("tmux", wArgs...); err != nil {
			return fmt.Errorf("creating tmux window %s: %w", w.Name, err)
		}

		if err := sendWindowCommand(r, name, w, cmdArgs); err != nil {
			return err
		}
		if err := createPanes(r, name, w, path, cmdArgs); err != nil {
			return err
		}
	}

	for _, w := range windows {
		if idx, ok := focusedPaneIndex(w); ok {
			target := fmt.Sprintf("%s:%s.%d", name, w.Name, idx+baseIdx)
			if _, err := r.Run("tmux", "select-pane", "-t", target); err != nil {
				return fmt.Errorf("selecting pane in window %s: %w", w.Name, err)
			}
		}
	}

	focusWindow := first.Name
	for _, w := range windows {
		if w.Focus {
			focusWindow = w.Name
		}
	}
	if _, err := r.Run("tmux", "select-window", "-t", name+":"+focusWindow); err != nil {
		return fmt.Errorf("selecting window: %w", err)
	}

	return nil
}

// sendWindowCommand sends the composed command to a window via send-keys.
// When panes are configured, the first pane's command replaces the window command.
func sendWindowCommand(r exec.Runner, session string, w config.WindowConfig, cmdArgs string) error {
	cmd := w.Command
	if len(w.Panes) > 0 {
		cmd = w.Panes[0].Command
	}

	full := BuildCommand(w.EnvFile, w.Env, cmd, cmdArgs)
	target := session + ":" + w.Name
	if _, err := r.Run("tmux", "send-keys", "-t", target, full, "Enter"); err != nil {
		return fmt.Errorf("sending command to window %s: %w", w.Name, err)
	}
	return nil
}

// createPanes splits the window into additional panes.
// The first pane's command is already handled by sendWindowCommand;
// subsequent entries create splits.
func createPanes(r exec.Runner, session string, w config.WindowConfig, path, cmdArgs string) error {
	if len(w.Panes) <= 1 {
		return nil
	}

	target := session + ":" + w.Name

	for _, p := range w.Panes[1:] {
		// -h = horizontal layout (side-by-side), -v = vertical layout (stacked)
		splitFlag := "-v"
		if p.Split == "horizontal" {
			splitFlag = "-h"
		}

		splitArgs := []string{"split-window", splitFlag, "-t", target, "-c", path}
		if p.Size > 0 {
			splitArgs = append(splitArgs, "-l", fmt.Sprintf("%d%%", p.Size))
		}

		if _, err := r.Run("tmux", splitArgs...); err != nil {
			return fmt.Errorf("splitting pane in window %s: %w", w.Name, err)
		}

		// Panes inherit env/env_file from parent window
		full := BuildCommand(w.EnvFile, w.Env, p.Command, cmdArgs)
		if _, err := r.Run("tmux", "send-keys", "-t", target, full, "Enter"); err != nil {
			return fmt.Errorf("sending command to pane in %s: %w", w.Name, err)
		}
	}

	return nil
}

// paneBaseIndex returns the tmux pane-base-index setting.
// Falls back to 0 (the tmux default) if the option cannot be read.
func paneBaseIndex(r exec.Runner) int {
	out, err := r.Run("tmux", "show-option", "-gv", "pane-base-index")
	if err != nil {
		return 0
	}
	idx, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0
	}
	return idx
}

// focusedPaneIndex returns the index of the last pane with Focus set in a window.
// Returns false when no pane needs explicit focus (zero or one pane, or no focus set).
func focusedPaneIndex(w config.WindowConfig) (int, bool) {
	if len(w.Panes) <= 1 {
		return 0, false
	}
	idx := -1
	for i, p := range w.Panes {
		if p.Focus {
			idx = i
		}
	}
	if idx < 0 {
		return 0, false
	}
	return idx, true
}

// KillSession destroys a tmux session. Returns nil if the session doesn't exist.
func KillSession(r exec.Runner, name string) error {
	name = SanitizeName(name)
	if !SessionExists(r, name) {
		return nil
	}
	if _, err := r.Run("tmux", "kill-session", "-t", name); err != nil {
		return fmt.Errorf("killing tmux session %s: %w", name, err)
	}
	return nil
}

// CurrentSessionName returns the name of the tmux session that the current
// process is running in. Returns "" if not inside tmux or on error.
func CurrentSessionName(r exec.Runner) string {
	if !IsInsideSession() {
		return ""
	}
	out, err := r.Run("tmux", "display-message", "-p", "#{session_name}")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// SwitchToLastSession switches the tmux client to the previous session.
// Returns an error if there is no previous session to switch to.
func SwitchToLastSession(r exec.Runner) error {
	_, err := r.Run("tmux", "switch-client", "-l")
	if err != nil {
		return fmt.Errorf("switching to last session: %w", err)
	}
	return nil
}

// Connect connects to a tmux session. If already inside tmux, it
// switches the client; otherwise it attaches.
func Connect(r exec.Runner, name string) error {
	name = SanitizeName(name)
	if IsInsideSession() {
		return r.RunInteractive("tmux", "switch-client", "-t", name)
	}
	return r.RunInteractive("tmux", "attach-session", "-t", name)
}

// ListSessions returns the names of all active tmux sessions.
func ListSessions(r exec.Runner) ([]string, error) {
	out, err := r.Run("tmux", "list-sessions", "-F", "#{session_name}")
	if err != nil {
		// tmux exits non-zero when no server is running
		if strings.Contains(err.Error(), "no server running") || strings.Contains(err.Error(), "no current client") {
			return nil, nil
		}
		return nil, fmt.Errorf("listing tmux sessions: %w", err)
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// BuildCommand constructs the shell command string for a window or pane,
// prepending env_file sourcing and env var exports.
// A clear is always inserted so the user sees a clean terminal:
//   - With command: env setup && clear && command
//   - Without command: env setup && clear
//   - Nothing at all: clear
//
// The command may contain {{.Args}} which is replaced with the provided args.
// When args is empty the placeholder is removed and surrounding whitespace is trimmed.
func BuildCommand(envFile string, env map[string]string, command, args string) string {
	var parts []string

	if envFile != "" {
		parts = append(parts, fmt.Sprintf("set -a && source %s && set +a", shellQuote(envFile)))
	}

	if len(env) > 0 {
		keys := make([]string, 0, len(env))
		for k := range env {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var exports []string
		for _, k := range keys {
			exports = append(exports, fmt.Sprintf("%s=%s", k, shellQuote(env[k])))
		}
		parts = append(parts, "export "+strings.Join(exports, " "))
	}

	parts = append(parts, "clear")

	if command != "" {
		command = replaceArgs(command, args)
		if command != "" {
			parts = append(parts, command)
		}
	}

	return strings.Join(parts, " && ")
}

// replaceArgs substitutes {{.Args}} in a command string with the given args.
// When args is empty the placeholder is removed and surrounding whitespace is collapsed.
func replaceArgs(command, args string) string {
	if !strings.Contains(command, "{{.Args}}") {
		return command
	}
	replaced := strings.ReplaceAll(command, "{{.Args}}", args)
	// Collapse multiple spaces left by empty replacement and trim edges.
	parts := strings.Fields(replaced)
	return strings.Join(parts, " ")
}

// shellQuote wraps s in single quotes if it contains shell-unsafe characters.
func shellQuote(s string) string {
	for _, c := range s {
		if !isSafeShellChar(c) {
			s = strings.ReplaceAll(s, "'", `'\''`)
			return "'" + s + "'"
		}
	}
	return s
}

func isSafeShellChar(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
		c == '-' || c == '_' || c == '.' || c == '/' || c == ':' || c == '='
}
