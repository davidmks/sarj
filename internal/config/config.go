// Package config handles loading and merging global and per-project TOML configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Config holds the merged configuration from global and per-project sources.
type Config struct {
	WorktreeBase  string `toml:"worktree_base"`
	DefaultBranch string `toml:"default_branch"`
	AutoAttach    bool   `toml:"auto_attach"`

	Tmux TmuxConfig `toml:"tmux"`

	// Per-project only fields
	SetupCommand string   `toml:"setup_command"`
	SetupAsync   *bool    `toml:"setup_async"`
	SetupClose   *bool    `toml:"setup_close"`
	Symlinks     []string `toml:"symlinks"`
}

// IsSetupAsync returns whether the setup command should run asynchronously
// in a tmux window. Defaults to false when not explicitly configured.
func (c *Config) IsSetupAsync() bool {
	return c.SetupAsync != nil && *c.SetupAsync
}

// ShouldCloseSetup returns whether the async setup window should auto-close
// on success. Defaults to true when not explicitly configured.
func (c *Config) ShouldCloseSetup() bool {
	if c.SetupClose == nil {
		return true
	}
	return *c.SetupClose
}

// TmuxConfig holds tmux-related settings.
type TmuxConfig struct {
	Enabled bool           `toml:"enabled"`
	Windows []WindowConfig `toml:"windows"`
}

// WindowConfig describes a single tmux window to create.
type WindowConfig struct {
	Name    string            `toml:"name"`
	Command string            `toml:"command"`
	EnvFile string            `toml:"env_file"`
	Env     map[string]string `toml:"env"`
	Panes   []PaneConfig      `toml:"panes"`
	Focus   bool              `toml:"focus"`
}

// PaneConfig describes a split within a tmux window.
// Panes inherit env/env_file from their parent window.
type PaneConfig struct {
	Command string `toml:"command"`
	Split   string `toml:"split"` // "horizontal" or "vertical" (default: vertical)
	Size    int    `toml:"size"`  // percentage (default: 50)
	Focus   bool   `toml:"focus"`
}

// Defaults returns a Config with sensible zero-config defaults.
func Defaults(repoName string) Config {
	return Config{
		WorktreeBase:  "~/wt/" + repoName,
		DefaultBranch: "main",
		AutoAttach:    true,
		Tmux: TmuxConfig{
			Enabled: true,
			Windows: []WindowConfig{
				{Name: "terminal"},
			},
		},
	}
}

// GlobalPath returns the default global config file path.
func GlobalPath() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "sarj", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}
	return filepath.Join(home, ".config", "sarj", "config.toml"), nil
}

// ProjectPath returns the per-project config file path for a given repo root.
func ProjectPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".sarj.toml")
}

// LocalPath returns the per-user, per-project config file path.
// This file should be gitignored and never committed.
func LocalPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".sarj.local.toml")
}

// Load reads global, per-project, and local configs, merges them, and applies defaults.
// repoRoot is the git repository root; repoName is used for template expansion.
func Load(repoRoot, repoName string) (*Config, error) {
	globalPath, err := GlobalPath()
	if err != nil {
		return nil, err
	}
	return LoadWithPaths(globalPath, ProjectPath(repoRoot), LocalPath(repoRoot), repoName)
}

// LoadWithPaths is like Load but accepts explicit file paths (for testing).
func LoadWithPaths(globalPath, projectPath, localPath, repoName string) (*Config, error) {
	cfg := Defaults(repoName)

	if err := loadFile(globalPath, &cfg); err != nil {
		return nil, fmt.Errorf("loading global config: %w", err)
	}

	var proj Config
	if err := loadFile(projectPath, &proj); err != nil {
		return nil, fmt.Errorf("loading project config: %w", err)
	}

	merge(&cfg, &proj)

	var local Config
	if err := loadFile(localPath, &local); err != nil {
		return nil, fmt.Errorf("loading local config: %w", err)
	}

	mergeLocal(&cfg, &local)

	if err := validateWindows(cfg.Tmux.Windows); err != nil {
		return nil, err
	}

	expanded, err := expandPath(cfg.WorktreeBase, repoName)
	if err != nil {
		return nil, err
	}
	cfg.WorktreeBase = expanded

	return &cfg, nil
}

// validateWindows checks that window and pane fields contain valid values.
func validateWindows(windows []WindowConfig) error {
	for _, w := range windows {
		for _, p := range w.Panes {
			switch p.Split {
			case "", "horizontal", "vertical":
			default:
				return fmt.Errorf("invalid pane split %q in window %q: must be \"horizontal\" or \"vertical\"", p.Split, w.Name)
			}
		}
	}
	return nil
}

// loadFile reads a TOML file into dst. Returns nil if the file doesn't exist.
func loadFile(path string, dst any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return toml.Unmarshal(data, dst)
}

// merge overlays per-project fields onto the global config.
// Per-project wins for: default_branch, setup_command, symlinks, tmux windows.
func merge(global, project *Config) {
	if project.DefaultBranch != "" {
		global.DefaultBranch = project.DefaultBranch
	}
	if project.SetupCommand != "" {
		global.SetupCommand = project.SetupCommand
	}
	if project.SetupAsync != nil {
		global.SetupAsync = project.SetupAsync
	}
	if project.SetupClose != nil {
		global.SetupClose = project.SetupClose
	}
	if len(project.Symlinks) > 0 {
		global.Symlinks = project.Symlinks
	}
	if len(project.Tmux.Windows) > 0 {
		global.Tmux.Windows = project.Tmux.Windows
	}
}

// mergeLocal overlays local (per-user, per-project) fields onto the merged config.
// Unlike merge, local can override any section including tmux.
func mergeLocal(base, local *Config) {
	if local.WorktreeBase != "" {
		base.WorktreeBase = local.WorktreeBase
	}
	if local.DefaultBranch != "" {
		base.DefaultBranch = local.DefaultBranch
	}
	if local.SetupCommand != "" {
		base.SetupCommand = local.SetupCommand
	}
	if local.SetupAsync != nil {
		base.SetupAsync = local.SetupAsync
	}
	if local.SetupClose != nil {
		base.SetupClose = local.SetupClose
	}
	if len(local.Symlinks) > 0 {
		base.Symlinks = local.Symlinks
	}
	if len(local.Tmux.Windows) > 0 {
		base.Tmux.Windows = local.Tmux.Windows
	}
}

// expandPath replaces ~ with $HOME and {{.RepoName}} with the repo name.
func expandPath(path, repoName string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("determining home directory: %w", err)
		}
		path = filepath.Join(home, path[2:])
	}
	return strings.ReplaceAll(path, "{{.RepoName}}", repoName), nil
}
