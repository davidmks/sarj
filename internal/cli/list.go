package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/davidmks/sarj/internal/exec"
	"github.com/davidmks/sarj/internal/git"
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
			mainWT, _ := git.MainWorktree(r)

			entries := make([]listEntry, 0, len(wts))
			for _, wt := range wts {
				if wt.Path == mainWT {
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

			if output == "json" {
				return printJSON(cmd.OutOrStdout(), entries)
			}
			return printText(cmd.OutOrStdout(), entries)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "text", "output format: text|json")

	return cmd
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

func printText(w io.Writer, entries []listEntry) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tBRANCH\tAHEAD/BEHIND\tAGE\tDIRTY\tTMUX") //nolint:errcheck
	for _, e := range entries {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", //nolint:errcheck
			e.Name, e.Branch,
			formatAheadBehind(e.Upstream),
			formatAge(e.Head.Date),
			formatDirty(e.Dirty),
			formatTmux(e.Tmux),
		)
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
