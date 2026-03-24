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

> **macOS:** Apple may show a security warning because the binary is unsigned. To allow it:
> ```
> xattr -d com.apple.quarantine $(which sarj)
> ```

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

**`sarj delete`**
- Kill tmux session
- Remove worktree
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

### Per-project: `.sarj.toml`

Team-shared settings — setup command, symlinks, default branch.

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

### `sarj delete <name> [flags]`

Remove a worktree and kill its tmux session.

| Flag | Description |
|------|-------------|
| `-D, --delete-branch` | Delete the branch (no prompt) |
| `--keep-branch` | Keep the branch (no prompt) |

### `sarj list`

List worktrees with branch and tmux session status.

### `sarj init [--global | --local]`

Generate a config file with commented defaults.

## Shell completions

sarj supports bash, zsh, and fish. Run `sarj completion <shell> --help` for setup instructions.

## License

MIT
