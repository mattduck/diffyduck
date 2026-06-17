# Plan: Split into three tools — `dfd`, `tdb`, `rpt`

Status: in progress (2026-06-17)

## Progress

- **P0 done** — module renamed to `github.com/mattduck/diffyduck`.
- **P1 done** — `pkg/comments`→`pkg/ticketdb`; comment/note CLI extracted to
  `pkg/ticketcli` (styles + highlight seam decoupled); new cgo-free `cmd/tdb`;
  dfd delegates comment/note to ticketcli. Parser covered by ticketcli tests.
  - *Deferred (optional):* dfd still parses comment flags in `main.go` for
    cross-command flag validation + early errors before delegating raw args to
    ticketcli (the single execution parser). Fully removing dfd's comment
    parser/usage/completion would change misuse error messages on other
    commands, so it's left as a later cleanup, not a blocker.
- **P2 done** — reviewparrot ported in: `pkg/scanner`, `pkg/rpconfig` (renamed
  from its `config` to avoid colliding with diffyduck's `pkg/config`), `cmd/rpt`.
  `BurntSushi/toml` now direct. Makefile builds all three binaries + `cgo-free`
  gate wired into `check`. Three binaries (dfd/tdb/rpt) green.

Remaining: **P3-P5** (new features + polish) below — not yet started.

---

## Goal

Turn this repo into a single Go module hosting **three binaries**, all clients of a
shared git-backed comment/note ("ticket") state plus a shared code-comment scanner:

| Binary | Name        | Role                                                                 |
|--------|-------------|---------------------------------------------------------------------|
| `dfd`  | diffyduck   | Git/diff TUI. A *frontend* to the state; otherwise unchanged.        |
| `tdb`  | ticketdb    | CLI over the state: comments/notes/tickets **and** code-TODO markers.|
| `rpt`  | reviewparrot| LLM-driven review (ex-`revparrot`). Generic lint-style rules **and** situation-specific reviews; violations in code **and** state.|

Shared core: `pkg/ticketdb` (git-state store), `pkg/scanner` (code-comment parser),
`pkg/config` (rules/globs), `pkg/git`.

## Locked decisions (from kickoff Q&A)

- **Module path:** rename `github.com/user/diffyduck` → `github.com/mattduck/diffyduck`.
  Repo/module stays `diffyduck`; the three tools live under it.
- **Third tool:** `ticketdb` / binary `tdb`.
- **Review tool:** keep the name **`reviewparrot` / binary `rpt`** (not "lintparrot/lpt").
  "Review" is deliberately broader than "lint": beyond generic, context-free rules
  (ruff/flake8-style), it should support *situation-specific* reviews — e.g. a review
  scoped to a diff/PR, a particular subsystem, or a one-off prompt — not just a fixed
  rule catalogue. The config/skill model must leave room for both (see P4 item 5).
- **State package name:** rename `pkg/comments` → **`pkg/ticketdb`**. We will have two
  unrelated notions of "comment" — the git-backed dfd item *and* parsed source-code
  comments — so the git-state store gets the `ticketdb` name. Code comments live only in
  `pkg/scanner` (as markers/violations, never typed `Comment`); the package boundary
  keeps the two apart. (Type rename `Comment`→`Ticket` deferred — see P3.)
- **CGO isolation:** `tdb` and `rpt` **must compile with `CGO_ENABLED=0`**. tree-sitter
  (cgo) is a `dfd`-only dependency. Verified today: `pkg/comments`/store, `pkg/git`, and
  revparrot's `pkg/scanner`+`pkg/config` already build cgo-free. The *only* leak is the
  comment block formatter syntax-highlighting code context via `pkg/highlight` — P1
  removes that via dependency injection.
- **Scope:** full vision, but **phased** (P0–P2 structural & behavior-preserving;
  P3–P4 new features; P5 polish). Each phase ends green (`make check`).
- **State ref:** keep `refs/dfd/comments` — no migration, no rename.

## Source material

- This repo: `pkg/comments/` (already isolated: `Store`, `Comment`, `Index`, `Matcher`),
  comment/note CLI **embedded** in `cmd/dfd/main.go` (~`runComment*`/`runNote*`,
  lines ~1897–3130), TUI consumes `pkg/comments` directly.
- `/f/2026-06-13-reviewparrot` (`github.com/mattduck/revparrot`, 2 commits):
  `pkg/scanner` (REVP/NOREVP multi-language comment parser, 20+ langs),
  `pkg/config` (TOML rules + glob include/exclude/ignore), `cmd/revparrot`
  (`check`, `rules`), skills `parrot-review` / `parrot-fix`. No git, no TUI.
  Only extra dep: `BurntSushi/toml` (already an *indirect* dep here).

## Target layout

```
github.com/mattduck/diffyduck
├── cmd/
│   ├── dfd/            # TUI (slimmed: comment CLI removed)
│   ├── tdb/            # ticketdb CLI (new)
│   └── rpt/            # reviewparrot CLI (ported from cmd/revparrot)
├── pkg/
│   ├── ticketdb/       # git-state store (renamed from pkg/comments; cgo-free)
│   ├── ticketcli/      # NEW: ticket/note CLI logic + list/block formatting (cgo-free)
│   ├── scanner/        # code-comment marker parser (from revparrot, generalized in P3)
│   ├── config/         # rules/globs (from revparrot)
│   ├── git/ content/ diff/ sidebyside/ highlight/ inlinediff/ pager/   # as-is
│   └── ...
├── internal/tui/       # TUI only; no longer the home of comment-CLI styles
├── skills/             # reconciled set (note/review/review-fix + parrot-*)
└── Makefile            # multi-binary build
```

---

## Phase 0 — Module rename & prep

Behavior-preserving; pure mechanical churn. Do this first so later diffs are clean.

1. `go.mod`: `module github.com/user/diffyduck` → `github.com/mattduck/diffyduck`.
2. Rewrite all import paths: `grep -rl 'github.com/user/diffyduck' --include='*.go' | xargs sed -i '' 's#github.com/user/diffyduck#github.com/mattduck/diffyduck#g'`.
3. `make check` green. Single commit: `refactor: rename module to github.com/mattduck/diffyduck`.

**Exit:** identical behavior, new module path everywhere.

---

## Phase 1 — Rename store, extract CLI → `pkg/ticketdb` + `pkg/ticketcli` + `cmd/tdb`

The biggest structural step. The CLI logic is ~1.2k lines wedged in `cmd/dfd/main.go`,
reaches into `internal/tui` for styling, and pulls `pkg/highlight` (cgo) for block view.
Untangle all three, then give it its own cgo-free binary.

### 1a. Rename `pkg/comments` → `pkg/ticketdb`

Move the dir, rename `package comments` → `package ticketdb`, rewrite imports
(`grep -rl 'diffyduck/pkg/comments' | xargs sed`). Update the TUI and tests. Keep the
`Comment`/`Store`/`Index`/`Matcher` type names for now (the package boundary already
disambiguates from code-comments). Behavior unchanged; `make check` green before 1b.

### 1b. Break the `internal/tui` style coupling

The CLI formatters take `tui.CommentListStyles` (defined `internal/tui/view.go:213`,
built by `CommentListTheme()`). A CLI binary importing the TUI package is backwards.

- Move `CommentListStyles` + `CommentListTheme()` into `pkg/ticketcli`. `internal/tui`
  imports *that*, not vice-versa.
- Audit other `tui.*` references in the CLI functions (`formatCommentOneline`,
  `formatCommentBlock`, `styleCommentPath`, `CommentDisplayMode`/`CommentBranchFilter`)
  and relocate the CLI-side ones.

### 1c. Break the `pkg/highlight` (cgo) coupling — make highlighting injectable

`formatCommentBlock` / `highlightContext` call `highlight.New()` to colorize a comment's
code context. That is the **only** thing that would drag tree-sitter/cgo into `tdb`.

- Define a minimal interface in `pkg/ticketcli`, e.g.
  `type ContextHighlighter interface { Lines(path, content string) []string }` (or a
  `func` field). `formatCommentBlock` takes one; `nil` ⇒ plain (uncolored) context.
- `cmd/tdb` passes `nil` → stays cgo-free. `cmd/dfd` passes a `pkg/highlight`-backed
  adapter → keeps today's colored output.

### 1d. Move CLI logic into `pkg/ticketcli`

Relocate from `cmd/dfd/main.go`: `runComment`, `runCommentList`, `runCommentAdd`,
`runCommentAddStandalone`, `runCommentAddFile`, `runCommentEdit`, `runNote`,
`runNoteEdit`, plus formatting helpers and the comment-* fields of `parsedArgs`
(extract a focused options struct rather than dragging all of `parsedArgs`). Expose
`ticketcli.Run(args []string, w io.Writer, hl ContextHighlighter) (exitCode int, err error)`.

### 1e. New binary `cmd/tdb/main.go`

Thin `main` that parses args and calls `ticketcli.Run(..., nil)`. Mirror today's
subcommands for muscle memory: `tdb comment {list,add,edit,resolve,unresolve}`,
`tdb note {list,add,edit}`. (Terminology bridge to "ticket" handled in P3.)
Add a CI assertion: `CGO_ENABLED=0 go build ./cmd/tdb/`.

### 1f. Slim `cmd/dfd`

Remove the moved functions and their `comment`/`note` dispatch from `main.go`,
`printUsage`, `complete.go` (`subcommands`, `flagsForCmd`). The TUI keeps using
`pkg/ticketdb` directly — unchanged.

> Decide: does `dfd comment …` stay as a deprecated alias that shells to `ticketcli`,
> or is it removed outright? Recommend keeping thin aliases one release for muscle memory.

**Exit:** `dfd` and `tdb` build; `CGO_ENABLED=0 go build ./cmd/tdb/` succeeds; all
existing comment/note tests pass (moved alongside `pkg/ticketcli`); `make check` green.

---

## Phase 2 — Bring revparrot in as `rpt`

Behavior-preserving port; `rpt` does exactly what `revparrot` does today.

1. Copy `pkg/scanner/` and `pkg/config/` from revparrot into this module; rewrite their
   imports to `github.com/mattduck/diffyduck/...`.
2. Copy `cmd/revparrot/` → `cmd/rpt/`; rename binary string/usage `revparrot`→`reviewparrot`,
   keep subcommands `check`, `rules`.
3. `go.mod`: promote `BurntSushi/toml` from indirect to a direct require (`go mod tidy`).
4. Port revparrot's tests; reconcile any `package main` test-name collisions.
5. Skills: bring `parrot-review` / `parrot-fix` into `skills/` (see P5 for reconciling
   with existing `review` / `review-fix`). Copy `revparrot.toml` example into docs.

**Exit:** `rpt check` / `rpt rules` behave as `revparrot` did; three binaries build;
`make check` green. Retire the standalone reviewparrot repo (or leave a pointer commit).

---

## Phase 3 — New: `tdb` gains code-comment / TODO parsing

First genuinely new capability. `tdb` becomes a CLI over **two data sources**:
(a) git-state tickets (`pkg/ticketdb`), (b) in-code markers (`pkg/scanner`).

1. **Generalize `pkg/scanner`** beyond `REVP`/`NOREVP`. Today it hardcodes the REVP
   grammar; make the marker set configurable: `TODO`, `FIXME`, `HACK`, `XXX`, `NOTE`,
   plus `REVP` (rpt) as one registered marker family. Keep `rpt`'s exact REVP behavior
   intact by registering it as the default for that binary.
2. **`tdb todo` (or `tdb scan`)**: walk the tree, report code markers as
   `file:line: TODO message`, with `--marker`, `--file`, `--grep` filters mirroring the
   git-state list flags for consistency.
3. **Unify the data model surface**: present code markers and git-state tickets through
   a common listing shape so `tdb list` can show both (flag to scope to one source).
4. **Ticket/note expansion**: grow standalone `Comment` (`File==""`) toward "tickets" —
   status beyond resolved/unresolved (e.g. open/in-progress/closed), optional
   title/tags. Keep on-disk format backward compatible (it's serialized in
   `refs/dfd/comments`); additive fields only.

**Exit:** `tdb` lists/filters both code markers and git-state tickets; existing tickets
still parse; `rpt` REVP behavior unchanged. `make check` green.

---

## Phase 4 — New: `rpt` reads git-state violations

`rpt` violations can originate from **both** in-code REVP annotations and git-state
comments flagged as rule violations.

1. Let `pkg/ticketdb` carry an optional rule-code tag on a `Comment` (additive field).
2. `rpt check` merges: REVP markers from `pkg/scanner` **+** rule-tagged tickets from
   `pkg/ticketdb` (`Store.AllComments` filtered by tag), unified into the existing
   `scanner.Violation` output shape.
3. Reconcile suppression semantics (`NOREVP` is code-side; decide the state-side
   equivalent — likely "resolved" suppresses).
4. Honor `revparrot.toml` rule scope/ignore for state-sourced violations too.
5. **Situation-specific reviews** (the broader-than-lint goal): generalize the review
   skill beyond the fixed rule catalogue so a review can be scoped/parameterized at
   invocation — a diff/PR, a subsystem path, or an ad-hoc prompt — and still emit REVP
   markers + state tickets through the same `check` pipeline. Keep generic `[[rules]]`
   as one mode; add an ad-hoc/scoped review mode alongside it. (Design item, not a
   blocker for P2–P4 which stay rule-based.)

**Exit:** `rpt check` surfaces violations from code and state; exit codes unchanged
(0 clean / 1 violations / 2 error). `make check` green.

---

## Phase 5 — Polish: build, skills, docs, completion

1. **Makefile** → multi-binary: `build` loops `dfd tdb rpt` (`go build -o <bin> ./cmd/<bin>/`);
   same for `install`; keep `check`/`fmt`/`vet`/`lint`/`cover`/`update-golden`/`fetch-queries`.
   Add a `cgo-free` target wired into `check`:
   `CGO_ENABLED=0 go build ./cmd/tdb/ ./cmd/rpt/` — a regression gate so neither tool
   ever silently re-acquires a tree-sitter dependency.
2. **Skills reconciliation** — current `skills/`: `note`, `review`, `review-fix`,
   `sweep`, `feedback`; revparrot's: `parrot-review`, `parrot-fix`. Decide the canonical
   set (likely: keep `note` pointed at `tdb`; fold `review`/`review-fix` ↔ `parrot-*`).
3. **Docs**: update root `README.md` for three tools; per-tool usage. Add a
   `revparrot.toml` reference. Refresh `CLAUDE.md` (architecture section, the
   "Keeping Help & Config in Sync" checklists now span three binaries).
4. **Completions**: extend `cmd/dfd/complete.go` patterns to `tdb`/`rpt` (or per-binary
   completion files).

**Exit:** `make check` green; all three tools documented, completable, installable.

---

## Terminology map

| Today (dfd)                       | After                                             |
|-----------------------------------|---------------------------------------------------|
| `Comment` with `File != ""`       | inline comment (file-attached)                    |
| `Comment` with `File == ""` (note)| standalone note → **ticket** (P3 expands status)  |
| `refs/dfd/comments`               | unchanged (shared state ref)                      |
| `dfd comment …` / `dfd note …`    | `tdb comment …` / `tdb note …` (+ alias in dfd?)  |
| revparrot `REVP(code)`            | `rpt` REVP marker (one marker family in scanner)  |

## Risks / open questions

1. **CLI/TUI/highlight untangle (P1b/1c)** is the trickiest part — verify nothing else
   in the TUI silently depends on the moved style types, and that the injected-highlighter
   seam fully severs `cmd/tdb`'s path to `pkg/highlight` (confirm with the `CGO_ENABLED=0`
   gate, not just a passing build on a cgo-enabled machine).
2. **`dfd comment` aliases**: keep deprecated or remove? (recommend: keep one release.)
3. **`pkg/scanner` generalization (P3)** must not regress `rpt`'s exact REVP/NOREVP
   semantics — pin with the ported revparrot tests before refactoring.
4. **Ticket schema growth (P3/P4)**: additive-only to the `refs/dfd/comments` serialized
   format; write a round-trip test against an old-format blob.
5. **Skills overlap (P5)**: `review`/`review-fix` vs `parrot-review`/`parrot-fix` —
   needs an explicit canonical decision.
6. **Naming of `tdb` scan subcommand**: `tdb todo` vs `tdb scan` vs `tdb list --source=code`.

## Suggested commit sequence

P0 module-rename · P1a pkg/ticketdb rename · P1b/c styles+highlight seam ·
P1d/e/f ticketcli+tdb, slim dfd · P2 rpt port · P3 scanner-generalize+todo · P3 tickets ·
P4 state-violations · P5 make/cgo-gate/skills/docs — each its own green commit.
