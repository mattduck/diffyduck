# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

DiffyDuck is a terminal-based side-by-side diff viewer written in Go, using Bubble Tea for the TUI. It displays Git diffs with syntax highlighting (via tree-sitter), three-level file folding, and Vim-style navigation.

## Build & Development Commands

```bash
make build           # Build to ./dfd
make install         # Install to $GOPATH/bin
make test            # Run all tests
make test-v          # Verbose test output
make check           # fmt-check + vet + lint + test (full CI check)
make update-golden   # Update golden file snapshots after intentional view changes
make cover           # Generate coverage report
make fetch-queries   # Download tree-sitter query files from upstream
```

Run a single test:
```bash
go test -v ./internal/tui -run TestSpecificName
go test -v ./pkg/diff -run TestParser
```

## Architecture

### Data Flow
```
Git Command → Parse unified diff → Transform to LinePairs → TUI Model → Render
```

### Package Structure

- **`cmd/dfd/`** - Entry point, CLI parsing, command orchestration
- **`internal/tui/`** - Bubble Tea TUI (model, update, view, search, keys)
- **`pkg/diff/`** - Unified diff parsing (`Diff` → `File` → `Hunk` → `Line`)
- **`pkg/sidebyside/`** - Transform diffs to side-by-side pairs (`FilePair`, `LinePair`)
- **`pkg/content/`** - Lazy content fetching with caching
- **`pkg/highlight/`** - Tree-sitter syntax highlighting engine
- **`pkg/inlinediff/`** - Word-level diff highlighting within lines
- **`pkg/pager/`** - Stdin reading and ANSI stripping

### Key Patterns

**Bubble Tea Model/Update/View:** The TUI follows Bubble Tea's architecture. State lives in `Model` (model.go), mutations happen in `Update` (update.go), rendering in `View` (view.go).

**Lazy Loading:** File contents are only fetched when a file is expanded. The `Fetcher` caches results.

**Three-Level Folding:** Each file has a `FoldLevel`: Folded (header only), Normal (hunks with context), Expanded (full file content).

**Golden File Tests:** View rendering tests use snapshot comparisons in `internal/tui/testdata/`. Run `make update-golden` after intentional visual changes.

### Testing Approach

Tests are layered:
1. Unit tests for pure logic (diff parsing, transforms)
2. State transition tests (cursor, scroll, fold behavior in update_test.go)
3. Golden file snapshots (view rendering in view_test.go)
4. Integration tests (full pipeline)

Test state transitions, not rendered output - assert `model.scroll == 5`, not parsed screen content.

### Git Isolation in Tests

**CRITICAL: Tests must NEVER run raw `exec.Command("git", ...)` calls.** Inherited environment variables like `GIT_DIR` (set by pre-commit hooks) will redirect git commands to the project repo, creating commits, modifying refs, or corrupting state in the real working tree.

**In production code (`pkg/git/git.go`):** Always use `g.command(...)` or `g.commandWithEnv(...)` instead of `exec.Command("git", ...)`. These methods strip `GIT_DIR`, `GIT_WORK_TREE`, and `GIT_INDEX_FILE` when `Dir` is set, making `NewWithDir(tmpDir)` inherently safe.

**In test helpers:** Use `cleanGitEnv(os.Environ())` to build a sanitised environment. See `runGit()` / `runGitWithEnv()` in `pkg/git/git_test.go` and `cleanGitEnv()` in `pkg/comments/store_test.go` for the pattern. Always use `t.TempDir()` for test repos and `NewWithDir(tmpDir)` for `RealGit` instances.

**For TUI tests:** Use `MockGit` — no real git operations needed.

## Keeping Help & Config in Sync

When adding or changing features, update all related surfaces. Use these checklists:

**New keybinding:**
1. `KeyMap` struct + `defaultKeyMap` in `internal/tui/keys.go`
2. `BindingGroups()` in `keys.go` (in-app C-h help)
3. `ApplyKeysConfig()` + `DefaultKeysConfig()` in `keys.go`
4. Config struct (e.g. `NavigationKeys`) in `pkg/config/config.go`
5. `GenerateExample()` in `pkg/config/example.go`

**New theme field:**
1. `ThemeConfig` struct in `pkg/config/config.go`
2. `DefaultTheme` in `pkg/config/example.go`
3. `ApplyTheme()` in `internal/tui/view.go`
4. `GenerateExample()` in `pkg/config/example.go`

**New CLI subcommand:**
1. `usageXxx` const in `cmd/dfd/main.go`
2. `printUsage()` switch case in `cmd/dfd/main.go`
3. Entry in `usageGeneral` Commands section

**New CLI flag:**
1. Relevant `usageXxx` const for the subcommand
2. `usageGeneral` if it's a global or cross-command flag

## Commit Conventions

Use commitlint keywords: `feat`, `fix`, `refactor`, `test`, `docs`, `style`, `build`, `chore`, `ci`, `perf`

Commit message format:
```
<type>: <description>

Changes:
- Bullet points describing what changed

Context:
- Why we're making the change

Review:
- Key design decisions, tradeoffs, areas needing attention
```
