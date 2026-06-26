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

## tdb: RPT in default markers — future decision point

Currently `RPT` is not in `scanner.DefaultMarkers()`, so `tdb list` does not show RPT
annotations unless you pass `--marker RPT` explicitly. Whether to include RPT in the
defaults (and how to handle the `RequireCode`/suppress semantics in the unified list view)
is an open question. Leave for later once category usage settles.

## tdb: richer filtering for todo/comment list ✓ done

`tdb list` supports: `--grep` (case-insensitive substring), `--marker` (keyword filter),
`--file` (exact path or trailing-`/` prefix; not full glob), `--status`, `--rule`, `--type`,
`-n`, `-b` (branch). `tdb comment list` shares the same flag set. Glob-style path matching
was not implemented — `--file` is exact or prefix-match only.

## rpt/tdb: comment structure linting (like disco project)

Ability to enforce that in-code comment markers adhere to a declared format.
Possible approaches:
- Hardcode a set of structural rules into the tool (e.g. TODO must be `TODO(owner): text`, FIXME must include a ticket ref)
- Or allow `revparrot.toml` to declare a `format` regex per marker keyword

Open questions:
- Does this live in rpt (rule-based linting focus) or tdb (marker scanning focus)?
- How do violations surface — as `rpt check` output, or a separate `tdb lint` command?
- Reference: "disco" project for prior art on the structure-enforcement pattern.

**Note (2510ca5):** RPT annotations that have a type but no scope (e.g. `RPT refactor:`) are
currently allowed but flagged as a LATER: "add linting that flags RPT annotations with no
scope." That lint step would live here.

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
