# diffyduck

Three terminal tools for reading, reviewing, and annotating code changes.

## Tools

| Binary | Name         | Role |
|--------|--------------|------|
| `dfd`  | diffyduck    | Side-by-side diff/log TUI with syntax highlighting and Vim-style navigation |
| `tdb`  | ticketdb     | CLI over the git-backed comment/note store: list, add, edit, resolve |
| `rpt`  | reviewparrot | Rule-based code review linter: scans for `RPT` annotations and rule-tagged tickets |

All three share a common git-state store (`refs/dfd/comments`) for inline comments,
standalone notes, and rule-tagged review tickets.

## Install

`dfd` requires Go and a C compiler (tree-sitter). `tdb` and `rpt` are pure Go
(`CGO_ENABLED=0`).

```sh
make bootstrap  # Pull tree-sitter grammar files (required for dfd)
make install    # Compile and install all three binaries to $GOPATH/bin
```

## dfd

Side-by-side git diff and log viewer.

```sh
dfd               # Working-tree status (default)
dfd diff          # Staged + unstaged changes
dfd diff main     # Diff vs a branch
dfd show abc123   # Show a commit
dfd log           # Browse commit history
dfd --help        # Full usage
```

Press `C-h` inside dfd to see keybindings. Use `dfd config --init` to generate a
config file with theme and keybinding customisation.

## tdb

CLI over the git-backed ticket store. Tickets are stored in `refs/dfd/comments`
and shared with dfd's in-TUI comment view.

```sh
tdb list                          # List all tickets and in-code markers
tdb list --source state           # Git-state tickets only
tdb list --source code            # In-code markers (TODO/FIXME/HACK/…) only
tdb list --rule SEC-AUTH          # Filter by rule code (ticket tag or RPT scope)
tdb list --marker RPT --exit-code # Exit 1 if any RPT annotations remain (CI gate)
tdb comment add src/foo.go:42     # Add a comment at a file:line
tdb comment list                  # List comments
tdb comment resolve <id>          # Resolve a comment
tdb note add -m "remember this"   # Add a standalone note
tdb completion bash               # Shell completion script
```

## rpt

Rule-based reviewer. Rules are defined in `revparrot.toml`. The agent (Claude)
places `RPT type(scope):` annotations at rule violations. `rpt check` *validates*
those annotations — malformed, unknown scope, mismatched type — and exits
non-zero if any are broken; `rpt ls` reports the review surface (rules × in-scope
files). The inventory of outstanding work items lives in `tdb list` (both code
annotations and rule-tagged tickets); `rpt check` no longer lists them.

```sh
rpt rules                         # List defined rules
rpt ls                            # Show rules × in-scope files across the whole tree
rpt ls --json src/pages           # Machine-readable, scoped to a set of paths
rpt diff                          # Show rules × files touched by working-tree diff
rpt diff main..HEAD               # Same, scoped to a ref range
rpt diff --show abc123            # Same, scoped to a single commit
rpt check                         # Validate RPT annotations (exit 1 = problems found)
rpt check -select rpt-syntax      # Run only the syntax check
rpt completion bash               # Shell completion script
```

## Status

Personal tooling — tested on my own machine and workflow. Subject to change.

## Code

99.9% generated.
