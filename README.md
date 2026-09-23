# sarj

**/ˈʃɒrʲ/ — Hungarian for shoot, sprout, new growth from a living tree. Also: offspring, descendant.**

Git worktree + tmux session manager. One command to create an isolated worktree with symlinks, setup, and a pre-configured tmux session — one command to tear it all down. tmux is optional.

## Requirements

- **git** — sarj manages git worktrees
- **tmux** (optional) — for session management. Without tmux, sarj works as a pure worktree manager.

## Install

### Homebrew

```bash
brew install davidmks/tap/sarj
```

### Go

```bash
go install github.com/davidmks/sarj/cmd/sarj@latest
```

### Binary

Download the latest release from [GitHub Releases](https://github.com/davidmks/sarj/releases), extract it, and add the binary to your PATH.

## Quick start

```bash
# Create a worktree with a tmux session
sarj create feat/my-feature

# Create with auto-generated name
sarj create
# => Created worktree calm-spinning-oak

# Create from a specific base branch
sarj create feat/v2 -b feat/v1

# List worktrees and their tmux session status
sarj list

# Rename branch, directory, and tmux session together
sarj rename feat/my-feature feat/better-name

# Delete a worktree (kills tmux session, removes worktree)
sarj delete feat/my-feature

# Delete and also remove the branch
sarj delete feat/my-feature -D
```

## How it works

**`sarj create`**
- Fetch latest from remote
- Create worktree and branch
- Symlink shared files (`.env`, secrets, etc.)
- Run setup command — rolls back everything on failure
- Open tmux session with configured windows/panes

**`sarj rename`**
- Rename the branch
- Move the worktree directory alongside it — rolls the branch name back on failure
- Rename the tmux session

**`sarj delete`**
- Kill tmux session
- Remove worktree: move it into a `.sarj-trash` folder next to it, then delete its files in the background
- Optionally delete the branch

## Configuration

sarj works with zero configuration. To customize, use `sarj init`:

```bash
# Per-project config (commit to repo)
sarj init

# Global config (personal preferences)
sarj init --global

# Local config (per-user, per-project — gitignored)
sarj init --local
```

### Global: `~/.config/sarj/config.toml`

Personal preferences — worktree location, tmux windows, auto-attach behavior. `{{.RepoName}}` expands to the repository directory name.

```toml
worktree_base = "~/wt/{{.RepoName}}"
default_branch = "main"
auto_attach = true

[tmux]
enabled = true

[[tmux.windows]]
name = "terminal"
command = ""

[[tmux.windows]]
name = "editor"
command = "nvim ."
env_file = ".env.test"

[[tmux.windows]]
name = "script"
command = ""
env = { UV_ENV_FILE = ".env" }
```

**Environment variables**: Use `env_file` to source a file (all variables are exported) or `env` to set individual variables. Both can be combined — the file is sourced first, then individual vars are exported. Panes inherit environment from their parent window.

Windows can have panes for side-by-side layouts:

```toml
[[tmux.windows]]
name = "dev"

[[tmux.windows.panes]]
command = "make dev"
size = 70

[[tmux.windows.panes]]
command = "make test-watch"
split = "horizontal"
```

Use `focus = true` to control which window is active on attach (defaults to the first window):

```toml
[[tmux.windows]]
name = "editor"
command = "nvim ."

[[tmux.windows]]
name = "shell"
focus = true
```

Panes support `focus` too — select a specific pane instead of the last one created:

```toml
[[tmux.windows.panes]]
command = "nvim ."
focus = true

[[tmux.windows.panes]]
command = "make watch"
split = "horizontal"
size = 30
```

#### Command placeholders

Window and pane commands can include placeholders that are substituted at session creation:

| Placeholder | Source | Notes |
|------|--------|-------|
| `{{.Args}}` | The `--args` flag passed to `sarj create` | Shell-quoted to preserve word boundaries |
| `{{.SetupCommand}}` | The `setup_command` config value | Inlined as a shell snippet (not quoted) |

This makes it possible to keep the layout in your global config and let projects supply only the data they own. For example, the global config defines a 3-pane layout where the third pane runs the project's setup command:

```toml
# ~/.config/sarj/config.toml
[[tmux.windows]]
name = "dev"

[[tmux.windows.panes]]
command = "claude {{.Args}}"
focus = true

[[tmux.windows.panes]]
command = "nvim"
split = "horizontal"

[[tmux.windows.panes]]
command = "{{.SetupCommand}}"
split = "vertical"
size = 30
```

Each project then declares only its own `setup_command` in `.sarj.toml`, and the third pane runs whatever that project supplies. If a placeholder's value is empty, the placeholder is removed cleanly.

**Interaction with `--no-setup` and `auto_setup`:**

- `sarj create --no-setup` clears `{{.SetupCommand}}` (the placeholder resolves to empty) so the user's explicit opt-out also stops the tmux-driven run.
- `auto_setup = false` only skips the synchronous `setup_command` run; `{{.SetupCommand}}` still resolves to its value, which is the normal way to defer setup into a tmux pane.

### Per-project: `.sarj.toml`

Team-shared settings — setup command, symlinks, default branch, tmux windows.

```toml
default_branch = "trunk"
setup_command = "make setup"

symlinks = [
    ".env",
    ".env.secrets",
    "ssl",
    ".claude/settings.local.json",
]
```

#### Running setup asynchronously in tmux

By default, `setup_command` runs synchronously before the tmux session opens, blocking until it finishes. To skip the synchronous run and instead launch setup in tmux, set `auto_setup = false` and add either a dedicated window or a pane.

**As a window:**

```toml
setup_command = "make setup"
auto_setup = false  # equivalent to passing --no-setup every time

[[tmux.windows]]
name = "setup"
command = "make setup && exit"  # auto-closes on success; drop "&& exit" to keep open

[[tmux.windows]]
name = "editor"
command = "nvim ."
focus = true
```

**As a pane** (alongside your editor in the same window):

```toml
setup_command = "make setup"
auto_setup = false

[[tmux.windows]]
name = "dev"

[[tmux.windows.panes]]
command = "nvim ."
size = 70
focus = true

[[tmux.windows.panes]]
command = "make setup"
split = "horizontal"
```

Setup runs alongside your work instead of blocking. `--no-setup` only skips the synchronous `setup_command` run during `sarj create` (and forces it off even when `auto_setup = true`); it does not stop a tmux window or pane that has its own command. Use `--no-tmux` to skip the tmux session entirely.

#### Status hook

A shell command run per worktree. Trimmed stdout is the state. Non-zero exit, empty output, or timeout (default 10s, configurable) all map to `unknown`. Forge-agnostic — no `gh` dep, no caching.

```toml
[status]
command = "gh pr list --state all --head \"$BRANCH\" --json state -q '.[0].state // \"none\"' 2>/dev/null"
timeout = "10s"     # optional; accepts any time.ParseDuration value
```

`gh pr list --state all` is preferred over `gh pr view` so CLOSED-not-merged PRs are detected too. `--state` matches case-insensitively, so a hook returning `MERGED` matches `--state merged`.

The branch name and worktree path are exposed as environment variables so values aren't interpolated into the shell command (no quoting, no injection):

| Variable | Value |
|----------|-------|
| `$BRANCH` | Worktree branch name |
| `$SARJ_WT_PATH` | Worktree absolute path |

When configured, `sarj list` adds a `STATUS` column and populates the JSON `status` field, and `sarj delete --state merged` filters by the token. When unset, the column is omitted, JSON `status` is `null`, and `--state` errors. Compose with `jq` for ad-hoc filters:

```sh
sarj list -o json | jq -r '.[] | select(.status=="merged") | .name' | xargs sarj delete -y
```

### Local: `.sarj.local.toml`

Per-user overrides for a specific project — gitignored, never committed. Sections defined here replace the corresponding section from the project or global config. Generate with `sarj init --local`.

When multiple configs exist: **Global → Project → Local**, each layer overrides the one above for sections it defines.

## Commands

### `sarj create [name] [flags]`

Create a worktree with optional tmux session.

| Flag | Description |
|------|-------------|
| `-b, --base <branch>` | Base branch (default: auto-detect) |
| `--no-setup` | Skip setup command |
| `--no-symlinks` | Skip symlinking |
| `--no-tmux` | Skip tmux session |
| `--no-attach` | Create session but don't attach |
| `--reuse-existing` | If the worktree already exists, reuse it and (re)build its tmux session instead of failing |

### `sarj delete [name...] [flags]`

Remove one or more worktrees and kill their tmux sessions. With no name, deletes the worktree at the current directory.

Delete returns right away, even for worktrees with large dependency folders. It moves each worktree into a `.sarj-trash` folder next to it, and a background process deletes the files afterwards. That folder is safe to delete by hand. Locked worktrees, and worktrees that can't be moved there, are removed in place by `git worktree remove`.

| Flag | Description |
|------|-------------|
| `-D, --delete-branch` | Delete the branch (no prompt) |
| `--keep-branch` | Keep the branch (no prompt) |
| `-y, --yes` | Skip prompts (defaults to keep-branch) |
| `--state <list>` | Filter by status hook output; repeat or comma-separate (e.g. `--state=merged,closed`) |

### `sarj rename [name] <new-branch> [flags]`

Rename a worktree's branch, directory, and tmux session in one step. With no name, renames the worktree at the current directory.

The directory moves to a sibling of where it is now, so a worktree kept outside `worktree_base` stays put. Uncommitted changes survive the move, and your shell and tmux panes follow the directory.

| Flag | Description |
|------|-------------|
| `--keep-path` | Keep the worktree directory where it is |
| `--no-tmux` | Skip renaming the tmux session |

Git does not rename the remote branch, so a branch that already tracks one keeps tracking it under the old name. sarj warns when that happens.

### `sarj list [-o text|json]`

List worktrees with branch, ahead/behind, last-commit age, dirty flag, and tmux session status. The `STATUS` column appears when a [status hook](#status-hook) is configured. `-o json` emits a stable schema for piping into `jq`.

### `sarj init [--global | --local]`

Generate a config file with commented defaults.

## Shell completions

sarj supports bash, zsh, and fish. Run `sarj completion <shell> --help` for setup instructions.

## License

MIT
