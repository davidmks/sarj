package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/davidmks/sarj/internal/config"
	"github.com/davidmks/sarj/internal/exec"
	"github.com/davidmks/sarj/internal/git"
	"github.com/davidmks/sarj/internal/status"
	"github.com/davidmks/sarj/internal/tmux"
	"github.com/davidmks/sarj/internal/worktree"
	"github.com/spf13/cobra"
)

type listEntry struct {
	Name     string        `json:"name"`
	Path     string        `json:"path"`
	Branch   string        `json:"branch"`
	Head     headInfo      `json:"head"`
	Upstream *upstreamInfo `json:"upstream"`
	Dirty    bool          `json:"dirty"`
	Tmux     *tmuxInfo     `json:"tmux"`
	Status   *string       `json:"status"`
}

type headInfo struct {
	SHA     string     `json:"sha"`
	Subject string     `json:"subject"`
	Date    *time.Time `json:"date"`
}

type upstreamInfo struct {
	Remote string `json:"remote"`
	Branch string `json:"branch"`
	Ahead  int    `json:"ahead"`
	Behind int    `json:"behind"`
}

type tmuxInfo struct {
	Session string `json:"session"`
	Active  bool   `json:"active"`
}

func newListCmd(r exec.Runner) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active worktrees",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if output != "text" && output != "json" {
				return fmt.Errorf("invalid -o value %q (want text or json)", output)
			}

			wts, err := worktree.List(r)
			if err != nil {
				return err
			}
			mainPath, _ := git.MainWorktree(r)

			cfg, err := loadListConfig(r, mainPath)
			if err != nil {
				return err
			}

			entries := make([]listEntry, 0, len(wts))
			for _, wt := range wts {
				if wt.Path == mainPath {
					continue
				}
				entries = append(entries, listEntry{
					Name:   filepath.Base(wt.Path),
					Path:   wt.Path,
					Branch: wt.Branch,
					Head:   headInfo{SHA: wt.HEAD},
				})
			}

			warnings := make([][]string, len(entries))
			var wg sync.WaitGroup
			for i := range entries {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					warnings[i] = enrichOne(r, &entries[i])
				}(i)
			}
			wg.Wait()

			stderr := cmd.ErrOrStderr()
			for _, ws := range warnings {
				for _, w := range ws {
					fmt.Fprintln(stderr, w) //nolint:errcheck
				}
			}

			if sessionSet := loadTmuxSessions(r); sessionSet != nil {
				for i := range entries {
					sn := tmux.SanitizeName(entries[i].Name)
					entries[i].Tmux = &tmuxInfo{Session: sn, Active: sessionSet[sn]}
				}
			}

			showStatus := cfg != nil && cfg.Status.Command != ""
			if showStatus {
				probeStatus(cmd.Context(), r, cfg.Status.Command, entries)
			}

			if output == "json" {
				return printJSON(cmd.OutOrStdout(), entries)
			}
			return printText(cmd.OutOrStdout(), entries, showStatus)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "text", "output format: text|json")

	return cmd
}

// loadListConfig loads the merged config when we're inside a repo. List used
// to work without config; it still does — config is only needed for the
// optional status hook, so a missing repo root resolves to a nil config.
func loadListConfig(_ exec.Runner, repoRoot string) (*config.Config, error) {
	if repoRoot == "" {
		return nil, nil
	}
	return config.Load(repoRoot, filepath.Base(repoRoot))
}

// probeStatus runs the configured status hook for each entry in parallel and
// fills in the Status field. Failures map to status.Unknown internally.
func probeStatus(ctx context.Context, r exec.Runner, command string, entries []listEntry) {
	if ctx == nil {
		ctx = context.Background()
	}
	items := make([]status.Item, len(entries))
	for i, e := range entries {
		items[i] = status.Item{Branch: e.Branch, Path: e.Path}
	}
	results := status.ProbeAll(ctx, r, command, items, 0)
	for i := range entries {
		entries[i].Status = &results[i].State
	}
}

// enrichOne fills in dirty, head subject/date, upstream, and ahead/behind for
// an already-base-populated entry. Per-call git failures are returned as
// warnings (caller drains them after wg.Wait); failed fields stay at zero.
func enrichOne(r exec.Runner, e *listEntry) []string {
	var warnings []string
	warn := func(err error) {
		warnings = append(warnings, fmt.Sprintf("warning: %s: %v", e.Name, err))
	}

	if dirty, err := git.Dirty(r, e.Path); err != nil {
		warn(err)
	} else {
		e.Dirty = dirty
	}

	if subject, date, err := git.HeadInfo(r, e.Path); err != nil {
		warn(err)
	} else {
		e.Head.Subject = subject
		e.Head.Date = &date
	}

	remote, branch, err := git.Upstream(r, e.Path)
	if err != nil {
		return warnings
	}
	up := &upstreamInfo{Remote: remote, Branch: branch}
	if ahead, behind, err := git.AheadBehind(r, e.Path, remote+"/"+branch); err != nil {
		warn(err)
	} else {
		up.Ahead = ahead
		up.Behind = behind
	}
	e.Upstream = up
	return warnings
}

// loadTmuxSessions returns a set of active tmux session names, or nil if tmux
// is unavailable on the system.
func loadTmuxSessions(r exec.Runner) map[string]bool {
	sessions, err := tmux.ListSessions(r)
	if err != nil {
		return nil
	}
	set := make(map[string]bool, len(sessions))
	for _, s := range sessions {
		set[s] = true
	}
	return set
}

func printText(w io.Writer, entries []listEntry, showStatus bool) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	header := "NAME\tBRANCH\tAHEAD/BEHIND\tAGE\tDIRTY\tTMUX"
	if showStatus {
		header += "\tSTATUS"
	}
	fmt.Fprintln(tw, header) //nolint:errcheck
	for _, e := range entries {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s", //nolint:errcheck
			e.Name, e.Branch,
			formatAheadBehind(e.Upstream),
			formatAge(e.Head.Date),
			formatDirty(e.Dirty),
			formatTmux(e.Tmux),
		)
		if showStatus {
			fmt.Fprintf(tw, "\t%s", formatStatus(e.Status)) //nolint:errcheck
		}
		fmt.Fprintln(tw) //nolint:errcheck
	}
	return tw.Flush()
}

func printJSON(w io.Writer, entries []listEntry) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

func formatAheadBehind(u *upstreamInfo) string {
	if u == nil {
		return "-/-"
	}
	return fmt.Sprintf("+%d/-%d", u.Ahead, u.Behind)
}

func formatAge(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "?"
	}
	d := time.Since(*t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 14*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 60*24*time.Hour:
		return fmt.Sprintf("%dw", int(d.Hours()/(24*7)))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy", int(d.Hours()/(24*365)))
	}
}

func formatDirty(dirty bool) string {
	if dirty {
		return "*"
	}
	return ""
}

func formatTmux(t *tmuxInfo) string {
	if t != nil && t.Active {
		return "active"
	}
	return "-"
}

func formatStatus(s *string) string {
	if s == nil || *s == "" {
		return "-"
	}
	return *s
}
