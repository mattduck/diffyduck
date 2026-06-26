# Plan

## rpt: rename REVP/NOREVP keywords to align with the binary name ✓ done

Renamed to RPT/NORPT.

## rpt: promote `show` to a top-level subcommand ✓ done

`rpt show [ref]` is now a top-level subcommand.

## rpt/tdb: annotation types (conventional-commit format) ✓ done

**Initial design** used `RPT(category:code):` syntax (cb66c2e), then evolved (2510ca5) to
align with conventional-commit style: `RPT type(scope): message`.

**Final format:** `RPT type(scope): message` — e.g. `RPT refactor(use-pathlib): use pathlib`.
`type` maps to conventional-commit keywords (feat, fix, refactor, perf, …). `scope` is the
rule code. Scope is optional for TODO/FIXME; RPT requires a structured form (`Strict=true`).

**Config:** `Rule` carries an optional `type` field (toml key `type`, was `category`):
```toml
[[rules]]
code = "use-pathlib"
type = "refactor"
```
Config type is a fallback: annotation type wins; config type used when no annotation type.

**tdb:** `tdb list --marker RPT` shows type+scope in the kind column as `RPT feat(auth)`.
RPT is not in the default markers; see "tdb: RPT in default markers" below.

**rpt check/rules/diff/show:** display renders `blue(type:)red(scope)` (verbose) or
`RPT(blue(type:)red(scope))` (oneline) via shared `ruleIDPlain`/`ruleIDStyled` helpers.

## rpt/tdb: filtering by type ✓ done

`--type <name>` filter on `rpt check`, `rpt rules`, `rpt diff`, `rpt show` and `tdb list`
(code markers only). Case-insensitive match against effective type (annotation wins, config
fallback). Was `--category` before 2510ca5. Completions not wired (known gap).

## tdb: RPT in default markers ✓ done

RPT added to `DefaultMarkers()` in loose (non-strict) mode so malformed annotations
appear in `tdb list` alongside TODO/FIXME. `markerForKeyword` simplified — all
user-supplied keywords use the loose form. `--exclude-marker` flag added so users can
narrow back to TODO/FIXME-only without enumerating `--marker TODO,FIXME` (4676a12).

## tdb: richer filtering for todo/comment list ✓ done

`tdb list` supports: `--grep` (case-insensitive substring), `--marker` (keyword filter),
`--file` (exact path or trailing-`/` prefix; not full glob), `--status`, `--rule`, `--type`,
`-n`, `-b` (branch). `tdb comment list` shares the same flag set. Glob-style path matching
was not implemented — `--file` is exact or prefix-match only.

## rpt: built-in annotation-quality rules + ruff-style rule selection

**Goal:** `rpt check` should identify RPT annotations that look like they target a rule
but are malformed or reference an unknown rule — so an LLM can find and fix them in one
pass without a separate command.

### Decided design

**Three built-in rules** hardcoded in the binary (not sourced from `revparrot.toml`),
all with an `rpt-` prefix:

| Code | What it flags |
|------|---------------|
| `rpt-syntax` | RPT annotation found via loose scan that fails the strict grammar — bare `RPT`, `RPT(old-form):`, `RPT without-colon`, etc. |
| `rpt-unknown-scope` | Strict-grammar match whose scope doesn't correspond to any rule in the loaded config. (Currently these are silently counted; this promotes them to violations.) |
| `rpt-type-mismatch` | Strict-grammar match whose type doesn't match the `type` field declared for that rule in config. |

These are **disabled by default** — `rpt check` without extra flags behaves exactly as
today. They live in the binary alongside user-config rules and participate in the same
violation output pipeline (file:line, rule block, `--oneline`, `--statistics`,
selection/filter flags all work on them).

**Rule selection flags** replace the existing `--rule` flag (no backwards-compat
constraint) across all `rpt` subcommands:

- `--select LIST` — run only these rule codes, replacing config rules entirely. Use to
  focus on a specific rule or a group of built-ins without loading config rules.
- `--extend-select LIST` — add these codes to the active config rules. The typical
  path for enabling built-ins: `rpt check --extend-select rpt-syntax,rpt-unknown-scope`.

LIST accepts comma-separated codes. To select all built-in `rpt-*` rules at once, a
group shorthand is TBD at impl (e.g. `rpt-*` glob or a named group like `rpt-builtins`).

**Why pre-selection matters for scanning efficiency:** `--select`/`--extend-select`
determine the active rule set *before* scanning starts. This enables the scan to be
scoped to only files relevant to the active rules (intersecting each rule's
include/exclude globs with the walk). With the old post-scan `--rule` filter, the full
tree was always walked regardless of which rule you cared about. This is especially
useful for `rpt diff` where the file set is already small.

**All three appear in `rpt check` output when selected.** The key distinction from
regular rules: a normal violation uses the scope parsed from the annotation as its rule
code (e.g. `v.Code = "use-pathlib"`). Built-in violations instead use the built-in rule
code itself (`v.Code = "rpt-syntax"`, `v.Code = "rpt-unknown-scope"`, etc.), and the
message describes what was found or wrong. The raw annotation content is never promoted
to the rule code position — it goes into the message. Examples:

- regular:         `RPT refactor(use-pathlib): ...`  →  `v.Code = "use-pathlib"`
- `rpt-syntax`:    `RPT blah blah` (no colon)        →  `v.Code = "rpt-syntax"`, message = `malformed annotation: "RPT blah blah"`
- `rpt-unknown-scope`: `RPT refactor(typo):` (no rule "typo") → `v.Code = "rpt-unknown-scope"`, message = `unknown scope "typo"`
- `rpt-type-mismatch`: `RPT fix(use-pathlib):` (rule has type "refactor") → `v.Code = "rpt-type-mismatch"`, message = `type "fix" does not match rule type "refactor"`

**Implementation notes:**
- `rpt-syntax` requires a second loose-mode RPT scan pass within the same global
  include/exclude scope; strict matches are subtracted to find the malformed-only set.
- `rpt-unknown-scope` is almost free — the current "skipped unknown" count logic
  already has the information; just emit violations instead.
- `rpt-type-mismatch` needs the config rule looked up per-violation; already done for
  display, so the field is available.
- Built-in rules are represented as `rpconfig.Rule` values constructed in the binary
  (not parsed from TOML), so `--select rpt-syntax`, `--statistics`, etc. work
  without special-casing.
- `--unknown` flag becomes redundant once `rpt-unknown-scope` is implemented (those
  annotations become proper violations rather than a count note). Deprecate it then.

### LATER: agent efficiency for multi-rule annotation checks

All three built-in rules start by finding RPT annotations — they differ only in what
aspect is validated. A naive implementation runs three separate scans and three separate
LLM check passes. Eventually we want a way for an agent to handle multiple rules that
share a "gather" phase in a single pass (one scan, one LLM call that checks all three
conditions). Design is TBD — requires some notion of rule groups or a multi-rule review
mode. Note this here so it isn't forgotten when the annotation rules are implemented.

## rpt: verbose mode for `rpt check` ✓ done

Default output is verbose blocks (rule + description, styled file path, code context,
message). Use `--oneline` for compact single-line output (f4fb469, 1e02a5d).

## rpt: output improvements for `rpt check` / `rpt diff`

- ✓ File paths are relative (to cwd or config root)
- ✓ Colour output: rule codes in bright red, file paths and line numbers styled;
  lipgloss respects NO_COLOR env and non-TTY automatically (f4fb469)
- ✓ Explicit `--color`/`--colour`/`--no-color`/`--no-colour` flags (af47640)
- Remove redundant file path repetition when multiple violations are in one file
  (group by file, print path once as a header) — **still open**
- Keyword in output — now intentional: `RPT type(scope)` encodes the annotation keyword
  and type together, so showing it in `--oneline` is meaningful. N/A.

## rpt: restrict check to config-defined rules by default ✓ done

When a config is present, `rpt check` only reports violations whose scope matches a
defined rule. Annotations with unknown scopes are counted and a note shown at the end:
"N annotations with unknown rule code skipped — use --unknown to include." When no config
is found, all annotations are reported (bc39e5d). Config option not added; `--unknown`
flag is the opt-out.

## rpt: `-n` flag for `rpt check` ✓ done

Renders up to N violations; summary line shows full count when truncated, e.g.
`Showing 5 of 23 violations.` (e4797c4).

## rpt: `--statistics` flag for `rpt check` ✓ done

`--statistics` shows a per-rule violation count table with rule title. Replaces
individual violation output; summary/skipped lines still shown after (720df07).
