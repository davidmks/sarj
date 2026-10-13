# sarj

**/ˈʃɒrʲ/: Hungarian for shoot, sprout, new growth from a living tree. Also: offspring, descendant.**

Git worktree + tmux session manager. One command to create an isolated worktree with symlinks, setup, and a pre-configured tmux session. One command to tear it all down. tmux is optional.

## Install

sarj needs **git**. **tmux** is optional: without it, sarj works as a pure worktree manager.

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

## Commands

### `sarj create [name] [flags]`

Create a worktree with optional tmux session. With no name, sarj generates one.

1. Fetch latest from remote
2. Create worktree and branch
3. Symlink shared files (`.env`, secrets, etc.)
4. Run setup command. If it fails, sarj rolls back everything.
5. Open tmux session with configured windows/panes

| Flag | Description |
|------|-------------|
| `-b, --base <branch>` | Base branch (default: `default_branch` from config) |
| `--args <string>` | Value for the `{{.Args}}` placeholder in tmux commands |
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
| `--state <list>` | Filter by [status hook](#status-hook) output; repeat or comma-separate (e.g. `--state=merged,closed`) |

### `sarj rename [name] <new-branch> [flags]`

Rename a worktree's branch, directory, and tmux session in one step. With no name, renames the worktree at the current directory.

The directory moves to a sibling of where it is now, so a worktree kept outside `worktree_base` stays put. If the move fails, sarj renames the branch back. Uncommitted changes survive the move, and your shell and tmux panes follow the directory.

| Flag | Description |
|------|-------------|
| `--keep-path` | Keep the worktree directory where it is |
| `--no-tmux` | Skip renaming the tmux session |

Git does not rename the remote branch, so a branch that already tracks one keeps tracking it under the old name. sarj warns when that happens.

### `sarj list [-o text|json]`

List worktrees with branch, ahead/behind, last-commit age, dirty flag, and tmux session status. The `STATUS` column appears when a [status hook](#status-hook) is configured. `-o json` prints JSON for piping into `jq`.

### `sarj init [--global | --local]`

Generate a config file with commented defaults.

## Configuration

sarj works with zero configuration. To customize, generate a config file with `sarj init`.

| File | Created by | Use for |
|------|------------|---------|
| `~/.config/sarj/config.toml` | `sarj init --global` | Personal preferences: worktree location, tmux layout, auto-attach |
| `.sarj.toml` | `sarj init` | Team settings, committed to the repo: setup command, symlinks, default branch |
| `.sarj.local.toml` | `sarj init --local` | Your own overrides for one project. Add it to `.gitignore`. |

sarj reads them in order: **Global → Project → Local**. Each file overrides the ones before it, but only for the keys it sets. A file that defines `tmux.windows` replaces the whole window list.

### Keys

| Key | Default | Set in | Description |
|-----|---------|--------|-------------|
| `worktree_base` | `~/wt/{{.RepoName}}` | global, local | Where worktrees go. `{{.RepoName}}` is the repository directory name. |
| `default_branch` | `main` | any | Base branch when `-b` is not given |
| `auto_attach` | `true` | global | Attach to the tmux session after `create` |
| `setup_command` | none | any | Shell command run in the new worktree |
| `auto_setup` | `true` | any | Run `setup_command` during `create`. See [running setup in tmux](#running-setup-in-tmux). |
| `symlinks` | none | any | Files and folders to symlink from the main checkout |
| `tmux.enabled` | `true` | global | Create a tmux session |
| `tmux.windows` | one `terminal` window | any | Windows to open. See [tmux layout](#tmux-layout). |
| `status.command` | none | any | See [status hook](#status-hook) |
| `status.timeout` | `10s` | any | Time limit for the status hook, e.g. `500ms`, `5s`, `1m` |

Example `.sarj.toml`:

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

### tmux layout

Each `[[tmux.windows]]` entry opens a window. `command` runs in it on start.

```toml
[[tmux.windows]]
name = "terminal"

[[tmux.windows]]
name = "editor"
command = "nvim ."
focus = true
```

`focus = true` controls which window is active on attach (defaults to the first window).

**Panes.** Windows can have panes for side-by-side layouts. `split` is `vertical` (default) or `horizontal`. `size` is a percentage (default 50). Panes support `focus` too, to select a specific pane instead of the last one created.

```toml
[[tmux.windows]]
name = "dev"

[[tmux.windows.panes]]
command = "nvim ."
size = 70
focus = true

[[tmux.windows.panes]]
command = "make test-watch"
split = "horizontal"
```

**Environment variables.** Use `env_file` to source a file (all variables are exported) or `env` to set individual variables. If you use both, sarj sources the file first, then exports the individual vars. Panes inherit environment from their parent window.

```toml
[[tmux.windows]]
name = "editor"
command = "nvim ."
env_file = ".env.test"

[[tmux.windows]]
name = "script"
env = { UV_ENV_FILE = ".env" }
```

### Command placeholders

Window and pane commands can include placeholders. sarj fills them in when it creates the session. If a placeholder's value is empty, sarj removes it.

| Placeholder | Source | Notes |
|------|--------|-------|
| `{{.Args}}` | The `--args` flag passed to `sarj create` | Shell-quoted to preserve word boundaries |
| `{{.SetupCommand}}` | The `setup_command` config value | Inlined as a shell snippet (not quoted) |

This lets you keep the layout in your global config and let each project supply only its own `setup_command`. For example, a global 3-pane layout where the third pane runs the project's setup command:

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

### Running setup in tmux

By default, `setup_command` runs before the tmux session opens, and `create` waits for it to finish. To run setup in a tmux window or pane instead, set `auto_setup = false` and add a window or pane that runs it:

```toml
setup_command = "make setup"
auto_setup = false

[[tmux.windows]]
name = "setup"
command = "make setup && exit"  # closes on success; drop "&& exit" to keep it open

[[tmux.windows]]
name = "editor"
command = "nvim ."
focus = true
```

You can also use `command = "{{.SetupCommand}}"` in a pane, as in the [placeholder example](#command-placeholders).

`auto_setup = false` and `--no-setup` differ in one way:

| | Skips setup before the session opens | Empties `{{.SetupCommand}}` |
|---|---|---|
| `auto_setup = false` | yes | no |
| `--no-setup` | yes | yes |

Neither one stops a window or pane that has the setup command written out in full. Use `--no-tmux` to skip the tmux session entirely.

### Status hook

A shell command that sarj runs for each worktree. Its trimmed output is the worktree's status. A non-zero exit, empty output, or timeout gives `unknown`.

```toml
[status]
command = "gh pr list --state all --head \"$BRANCH\" --json state -q '.[0].state // \"none\"' 2>/dev/null"
timeout = "10s"
```

The command reads these environment variables:

| Variable | Value |
|----------|-------|
| `$BRANCH` | Worktree branch name |
| `$SARJ_WT_PATH` | Worktree absolute path |

When configured, `sarj list` adds a `STATUS` column and fills the JSON `status` field, and `sarj delete --state merged` filters by status. The match ignores case, so a hook returning `MERGED` matches `--state merged`. When unset, the column is hidden, JSON `status` is `null`, and `--state` errors.

Combine with `jq` for other filters:

```sh
sarj list -o json | jq -r '.[] | select(.status=="merged") | .name' | xargs sarj delete -y
```

## Shell completions

sarj supports bash, zsh, and fish. Run `sarj completion <shell> --help` for setup instructions.

## License

MIT
