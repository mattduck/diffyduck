# diffyduck

Three terminal tools for reading, reviewing, and annotating code changes.

## Tools

| Binary | Name         | Role |
|--------|--------------|------|
| `dfd`  | diffyduck    | Side-by-side diff/log TUI with syntax highlighting and Vim-style navigation |
| `tdb`  | ticketdb     | CLI over the git-backed comment/note store: list, add, edit, resolve |
| `rpt`  | reviewparrot | Rule-based code review linter: scans for `RPT` annotations and rule-tagged tickets |

All three share a common git-state store (`refs/dfd/comments`) for inline comments,
standalone notes, and tagged review tickets (marker/type/scope).

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
tdb list                          # List all db entries and in-file comments
tdb list --store db               # db entries only (issues + comments)
tdb list --store file             # In-file comments (TODO/FIXME/HACK/…) only
tdb list --kind issue             # Standalone db issues only
tdb list --kind comment           # Entries attached to a code range only
tdb list --prefix RPT             # Filter by prefix keyword (any store)
tdb list --type refactor          # Filter by type (any store)
tdb list --scope SEC-AUTH         # Filter by scope/code (any store)
tdb list --ticket ABC-123         # Filter by external ticket ref (any store)
tdb list --json                   # Machine-readable JSON array (any store/filter)
tdb list --random                 # One item at random (respects all filters)
tdb list --prefix RPT --random -n5 # Five random RPT items to work on
tdb list --stats                  # Counts breakdown (store/kind/prefix/type/scope/ticket)
tdb list --stats-group scope      # Counts grouped by a single field
tdb list --prefix RPT --exit-code # Exit 1 if any RPT annotations remain (CI gate)
tdb comment add src/foo.go:42     # Add a db comment at a file:line
tdb comment add src/foo.go:9 --prefix RPT --type refactor --scope SEC-AUTH -m "…"
tdb comment list                  # List db comments
tdb comment resolve <id>          # Resolve a db entry
tdb note add -m "remember this"   # Add a standalone db issue
tdb completion bash               # Shell completion script
```

## rpt

Rule-based reviewer. Rules are defined in `revparrot.toml`. The agent (Claude)
places `RPT type(scope):` annotations at rule violations. `rpt check` *validates*
those annotations — malformed, unknown scope, mismatched type — and exits
non-zero if any are broken; `rpt ls` reports the review surface (rules × in-scope
files). The inventory of outstanding work items lives in `tdb list` (both code
annotations and tagged tickets); `rpt check` no longer lists them. rpt's rule
code maps onto a ticket's generic `scope` tag (`tdb comment add --scope <code>`),
so a violation can be recorded either as an in-code annotation or as a ticket.

Annotations are written in the file's comment syntax — a line comment
(`//`, `#`, `--`) or a block comment (`/* ... */`) where the language has one.
In JSX/TSX, use the block form `{/* RPT type(scope): msg */}`: a `//` between
JSX tags renders as literal text, not a comment.

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
