# Plan: terminology + CLI reshape for the tdb store

Drafted 2026-07-18.

Goal: settle a coherent vocabulary for the state/code entries `tdb` manages, and
reshape the CLI around it. The word "comment" had become overloaded (a code
comment in source vs a file-attached db entry), "ticket" read too formal for a
local git-backed store, and "marker" was a vague name for what is really a
keyword prefix. This plan fixes the model first, then the CLI.

Status: **design settled; implementation started.**

Progress:
- [x] **Phase 1 — storage model** (`pkg/ticketdb/comment.go`): `Comment.Marker`
  renamed to `Prefix` (serialized `PREFIX:`, legacy `MARKER:` still read); new
  `Ticket` field (serialized `TICKET:`). Internal read-sites updated
  (`list.go`, `commands.go`); user-facing flag names left unchanged this phase.
  Round-trip + legacy-key tests added. Full suite + vet green.
- [x] **Phase 2 — CLI vocabulary**: `--source`→`--store` (`state`/`code`→
  `db`/`file`, old values kept as aliases), `--kind note`→`issue`,
  `--marker`/`--exclude-marker`→`--prefix`/`--exclude-prefix`, `--ref`→
  `--commit`, new `--ticket` filter + add flag, stats dims
  (`store/kind/prefix/type/scope/ticket`), JSON `store`/`prefix`/`ticket`,
  `(n/a — file comments)` label, friendly error on `--store file --kind issue`.
  Usage text, completion, README, tests updated. `make check` green.
  Note: the `comment`/`note` write verbs still exist and `note` still sets the
  legacy internal `kind=note` (runStateList accepts both) — removed in Phase 3.
- [x] **Phase 3 — write-verb collapse**: removed the `comment`/`note` verbs;
  flat top-level commands are now `tdb add [file:line]` (kind inferred from the
  file:line's presence), `tdb edit|resolve|unresolve <ID>` (ID-addressed,
  kind-agnostic), plus the `tdb list` reader. Dropped `Options.Note`/`Kind`, the
  write-side `--kind`, and dead code (`runNote`/`runComment`/`runCommentList`/
  `prefixList`). `validate()` rewritten: reader-only flags point back to
  `tdb list`, add-only flags rejected on the ID ops. Updated `tdb`/`dfd`
  dispatch, usage, and completion. `make check` green.
- [ ] Phase 4 — scanner grammar (parse leading ticket token from file comments).

Chosen defaults for the open decisions: `--ref`→`--commit` (yes), strict `add`
inference with no `--standalone` override, friendly error on
`--store file --kind issue`, tool-name rename out of scope.

---

## The model

Two orthogonal axes describe every entry, plus a set of fields.

### Axes

| Axis | Values | Question it answers |
|---|---|---|
| **store** | `db` \| `file` | Where does it physically live — the git-ref store, or in a source file? |
| **kind** | `issue` \| `comment` | Standalone (issue), or concerns a code range (comment)? |

`kind` is **derived, not stored**: for a db entry, `kind=comment` iff it has a
`file:line`, else `kind=issue` (this is today's `IsStandalone()`). A file entry
is always a comment.

### The store × kind matrix

|  | **kind = issue** (standalone) | **kind = comment** (code range) |
|---|---|---|
| **store = db** | **db issue** | **db comment** |
| **store = file** | ✗ impossible | **file comment** |

Rule that generates the ✗: `store=file ⟹ kind=comment` — anything living in a
file inherently concerns its surrounding code, so there are no file issues.

### Fields

| Field | Meaning | db issue | db comment | file comment |
|---|---|---|---|---|
| **store** / **kind** | axes above (kind derived) | ✓ | ✓ | ✓ |
| **prefix** | keyword: `TODO`/`FIXME`/`RPT`/… (was "marker"); what makes a file comment discoverable and lets db⇄file translate | ✓ | ✓ | ✓ |
| **type** | classification: feat / bug / epic / … | ✓ | ✓ | ✓ |
| **scope** | free scope/code identifier | ✓ | ✓ | ✓ |
| **ticket** | external tracker ref — `ABC-123` (JIRA) or `#123` (GitHub) | ✓ | ✓ | ✓ (regex from message) |
| **id** | internal db identifier | ✓ | ✓ | ✗ (keyed by file:line) |
| **status** | open / in-progress / closed | ✓ | ✓ | ✗ (no representation) |
| **file anchor** | hash of line + surrounding context, used to re-find the code range after edits (was "anchor") | — (no range) | ✓ | ✗ (it *is* the line) |
| provenance | author / created / updated / branch / commit | ✓ | ✓ | ✗ |

Shared across all three: **store, kind, prefix, type, scope, ticket**. Everything
else (id, status, file anchor, provenance) is db-only, and file anchor is
db-comment-only.

### File-comment annotation grammar

A file comment carries all its fields inline. `ticket` is parsed by regex from
the start of the message:

```
<leader> PREFIX type(scope): [TICKET] message
```

Examples:
```
// RPT   refactor(auth): ABC-123 extract this into a helper
// TODO  fix(parser):     #123 handle empty input
// FIXME perf:            message with no ticket ref
```

- `TICKET` is optional; matched as `[A-Z]+-\d+` (JIRA) or `#\d+` (GitHub).
- `type` and `(scope)` already parse today; `ticket` is the new optional leading
  token of the message.

---

## CLI reshape

### Key realisation: the `comment`/`note` write verbs earn their keep in zero operations

There are five subcommands: `list`, `add`, `edit`, `resolve`, `unresolve`. Group
them by how they address an entry:

| Op | Addresses target by | Does kind matter? |
|---|---|---|
| **add** | supplied content | kind = **implied by `file:line` presence** |
| **edit / resolve / unresolve** | **ID** | no — the ID already picks the exact entry |
| **list** | filters | kind is a `--kind` filter |

Since `kind` is derived from the `file:line`, `add` never needed the verb to carry
it; and the ID-addressed ops are kind-agnostic (today `comment resolve X` and
`note resolve X` do identical things). So the `comment` vs `note` verb split
carries no information in any operation. **Collapse it.**

### Reshaped surface

```
Usage: tdb list [options]
       tdb add [file:line] [options]
       tdb edit|resolve|unresolve <ID>

add: file:line present -> db comment; omitted -> db issue.
     (file comments are scanned from source, never authored via CLI.)
  -m MESSAGE             text (else stdin)
  --commit REF           commit/branch/tag to attach to  (provenance; was --ref)
  --ticket REF           external tracker ref (ABC-123, #123)
  --author NAME
  --prefix KW / --type VALUE / --scope CODE

list: merges db issues/comments and in-file comments into one view:
  --store VALUE          all (default), db, file
  --kind VALUE           all (default), comment, issue
  --prefix LIST          filter by prefix keyword(s) (TODO, RPT, …); any store
  --exclude-prefix LIST  exclude these prefix keyword(s)
  --type VALUE           filter by type (feat, bug, epic, …); any store
  --scope CODE           filter by scope/code
  --ticket REF           filter by external ticket ref; any store
  --status VALUE         unresolved (default), resolved, all; db only
  --file PATH            filter by file (trailing / = prefix match)
  --grep TEXT            filter by text (case-insensitive)
  -n[N]                  limit combined rows (bare = all)
  --random               shuffle rows; return one (or -n N) at random
  --stats                counts breakdown (store/kind/prefix/type/scope/ticket)
  --stats-group FIELD    collapse --stats to one field
  --json / --exit-code / -b,--branch [NAME] / --all-branches
```

### The three listing recipes

| You want | Command |
|---|---|
| file comments | `tdb list --store file` |
| db comments | `tdb list --store db --kind comment` |
| db issues | `tdb list --store db --kind issue` |

`--store file` implies `kind=comment`; `--store file --kind issue` is
valid-but-always-empty (candidate for a friendly error rather than empty output).

---

## Rename map (old → new)

| Old | New |
|---|---|
| "ticket" (umbrella noun) | gone; reused as **ticket** = external tracker ref |
| `--source` axis | `--store` |
| source value `state` / `code` | `db` / `file` |
| `--kind` value `note` | `issue` |
| `--marker` / `--exclude-marker` | `--prefix` / `--exclude-prefix` |
| `marker` field (JSON / stats / add flag) | `prefix` |
| `--ref` (add) | `--commit` |
| `anchor` (internal) | `file anchor` |
| `tdb comment` / `tdb note` verbs (aliases `c`/`n`) | removed; folded into `tdb add` (kind inferred) + `tdb edit\|resolve\|unresolve <ID>` |
| `comment list` / `note list` | `tdb list --kind comment` / `tdb list --kind issue` |
| stats dims `source, marker, kind, type, scope` | `store, kind, prefix, type, scope, ticket` |
| — (new) | `--ticket` filter + `add --ticket` |

The `(n/a — markers)` stats label already added for the `kind` dimension should
generalise: db-only dimensions (`status`, `author`, …) show `(n/a — file
comments)` for file rows; `file anchor` etc. likewise.

---

## Open decisions

1. **`--ref` → `--commit`?** `--ref` (git commit/branch/tag attached = provenance)
   now sits next to `--ticket` (external ref) and both read as "a reference".
   Rename `--ref` to `--commit` to disambiguate. *(Assumed yes in the sketch
   above.)*

2. **Strict inference on `add`, no `--standalone` override.** If a `file:line` is
   present it's a comment, full stop — no flag to force a standalone issue that
   nonetheless records a location. Keep it strict (recommended).

3. **`--store file --kind issue`** → friendly error vs silent empty result.

4. **The binary is named `tdb` / "ticketdb".** With "ticket" repurposed to mean an
   external ref, the tool name and any "ticket" wording in help/README read oddly.
   Decide whether renaming the tool's self-description is in scope (probably a
   later, separate change — the binary name is load-bearing).

---

## Implementation surfaces (per CLAUDE.md sync points)

When executing, every rename touches these, keep them in sync:

**tdb:**
- `pkg/ticketdb/comment.go` — field names (`Marker`→`Prefix`, `Anchor`→file
  anchor), new `Ticket` field + serialize/parse (`MARKER:`→`PREFIX:`, add
  `TICKET:`), `IsStandalone()` stays as the kind derivation.
- `pkg/ticketcli/parse.go` — collapse `comment`/`note` into `add` +
  ID-addressed ops; usage text; flag renames; `--commit`, `--ticket`.
- `pkg/ticketcli/list.go` — `--source`→`--store` (`state`/`code`→`db`/`file`),
  `--kind` `note`→`issue`, `--marker`→`--prefix`, stats dimensions, JSON field
  names, `(n/a — …)` labels.
- `pkg/scanner/` — file-comment annotation grammar: parse optional leading
  `TICKET` token from the message; keyword is a `prefix`.
- `cmd/tdb/complete.go` — subcommands list, `flagsForCmd()`, `completeFlagValue()`
  enumerated values.
- README and any help text.

**rpt:** `rpt` reads the scanner + the store's `Scope` as its rule code. Confirm
`prefix`/`ticket` renames don't break `rpt ls`/`rpt check` assumptions
(`pkg/rpconfig`, `cmd/rpt`).

**Tests / golden files:** `make update-golden` after intentional view changes;
update `pkg/ticketcli` and `pkg/ticketdb` tests for new flag/field names.

**Migration:** existing db blobs use `MARKER:`; parser must still read old
`MARKER:` (back-compat) while writing `PREFIX:`, same additive pattern already
used for `HEAD:`→`BRANCH_HEAD:`.
