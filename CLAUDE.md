# CLAUDE.md

## Project
sarj — Git worktree + tmux session manager (Go CLI)
Reference: https://gist.github.com/davidmks/bcb14933a060c57dadc6c03e12678fc9
Plan: ~/.claude/plans/scalable-percolating-marble.md

## Commands
make build          # build binary to bin/sarj
make test           # go test ./...
make test-int       # go test -tags integration ./...
make lint           # golangci-lint run (v2 config format)
make fmt            # gofumpt -w .

## Go Conventions
- Exported symbols get doc comments starting with the symbol name
- Each package gets a `// Package foo ...` comment

## Testing
- Table-driven tests (`tt` loop with `t.Run`) for same-behavior-different-inputs
- Separate `func TestFoo` functions when setup or assertions diverge per case
- If the table struct needs `setup func()` or `verify func()` fields, break it into separate tests
- Integration tests (`//go:build integration`) are few and focused — test real component boundaries, not logic already covered by unit tests

## Workflow
- Build incrementally — one package/feature at a time, verify before moving on
- Commit atomically — one logical change per commit, but stay pragmatic (don't over-split trivially related changes)
- The gist is the spec — if implementation diverges from the gist behavior, the gist wins
- The plan is a guide, not gospel — flag improvements or issues rather than following it blindly
