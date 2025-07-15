# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

DiffyDuck is a Go-based terminal diff viewer with syntax highlighting and side-by-side display. It provides an interactive TUI for viewing git diffs with enhanced visualization.

## Common Commands

### Building and Testing
- `go build .` - Build the main binary
- `go test ./...` - Run all tests
- `./make.sh check` - Run comprehensive checks (gofmt, build, test)
- `./make.sh fix` - Auto-format code with gofmt

### Running the Application
- `./diffyduck` - View current git diff interactively (default behavior)
- `./diffyduck diff [args]` - Explicit git diff mode with optional git diff arguments
- `./diffyduck show [commit]` - View specific commit diff (defaults to HEAD)
- `./diffyduck pager` - Read diff from stdin (pager mode)
- `git diff | ./diffyduck pager` - Pipe any diff output to pager mode
- Don't call the main diffyduck diff/show commands: you can't run them as they require an interactive tty

## Architecture

### Core Components

1. **main.go** - Entry point handling command-line arguments and input sources
   - Supports three modes: stdin input, `diff` subcommand, `show` subcommand
   - Falls back to `git diff` when no input provided

2. **parser/** - Diff parsing and structure
   - `DiffParser` parses unified diff format into `FileDiff` and `Hunk` structures
   - Handles git diff headers and hunk boundaries

3. **git/** - Git integration and file retrieval
   - `FileRetriever` gets file content from working tree or specific commits
   - Supports different file types (text, binary, new, deleted)

4. **aligner/** - Diff alignment and word-level diffing
   - `DiffAligner` creates side-by-side aligned view of changes
   - Generates word-level diffs within modified lines using diffmatchpatch

5. **syntax/** - Syntax highlighting
   - Tree-sitter based highlighting for supported languages
   - Language detection via file extensions
   - Extensible language definition system

6. **ui/** - Terminal user interface
   - Bubble Tea TUI with viewport for scrolling
   - Keyboard navigation (j/k, g/G, q to quit)
   - Side-by-side rendering with syntax highlighting

7. **display/** - Rendering and styling
   - Lipgloss-based styling for colors and layout

### Data Flow

1. Input → Parser (unified diff) → FileDiff structures
2. FileDiff → Git retriever → File content
3. File content + diff → Aligner → AlignedLines
4. AlignedLines → Syntax highlighter → Styled content
5. Styled content → UI renderer → Terminal display

### Key Dependencies

- Bubble Tea: TUI framework
- Lipgloss: Terminal styling
- Tree-sitter: Syntax parsing
- go-diff/diffmatchpatch: Word-level diffing

## Testing

Tests are co-located with source files using `_test.go` suffix. Key test areas:
- Diff parsing in `parser/`
- Line alignment in `aligner/`
- Git operations in `git/`
- UI model behavior in `ui/`

## Development Tasks

- Add docs to all golang functions