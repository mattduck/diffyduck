# Plan

## rpt: rename REVP/NOREVP keywords to align with the binary name ✓ done

Renamed to RPT/NORPT.

## rpt: promote `show` to a top-level subcommand ✓ done

`rpt show [ref]` is now a top-level subcommand.

## rpt/tdb: annotation categories ✓ done

**Decision:** category is carried in the annotation using a `category:code` form inside the
parens: `RPT(refactor:use-pathlib): message`. Category is optional; `RPT(code): message`
is still valid and backward-compatible. The `:` separator was chosen over other delimiters
because it's already the RPT convention (the `):` closes the parens form).

**Config:** `Rule` gains an optional `category` field in `revparrot.toml`:
```toml
[[rules]]
code = "use-pathlib"
category = "refactor"
```
The config category is a fallback: if an annotation supplies its own category, that wins;
if not, the rule's config category is used in display output.

**tdb:** `tdb list --marker RPT` shows the category in the `kind` column as `RPT:category`
when present. RPT is not in the default markers; this is a point to revisit later (see
"tdb: RPT in default markers" below).

**rpt check:** verbose block output renders `blue(category:)red(code)` on the Rule line,
followed by the short title. Oneline output renders `RPT(blue(category:)red(code))` matching
the annotation syntax. Category is resolved from the violation annotation first, falling back
to the rule config, so state violations (which have no annotation syntax) also get category
info from config. `rpt rules`, `rpt diff`, and `rpt show` use the same `category:code`
convention via shared `ruleIDPlain`/`ruleIDStyled` helpers.

## rpt/tdb: filtering by category — next up

Add `--category <name>` filter to `rpt check`, `rpt rules`, `rpt diff`, `rpt show`, and
`tdb list --marker RPT`. For `rpt check` this narrows violations to those whose category
(annotation or config fallback) matches. For `rpt diff`/`rpt show` this scopes the rule
listing. For `tdb list` it filters marker rows. Design: case-insensitive match; multiple
values could be comma-separated or via repeated flags (decide at impl time).

## tdb: RPT in default markers — future decision point

Currently `RPT` is not in `scanner.DefaultMarkers()`, so `tdb list` does not show RPT
annotations unless you pass `--marker RPT` explicitly. Whether to include RPT in the
defaults (and how to handle the `RequireCode`/suppress semantics in the unified list view)
is an open question. Leave for later once category usage settles.

## tdb: richer filtering for todo/comment list

`tdb list` and `tdb comment list` should support filtering by:
- Text search / grep (case-insensitive substring or regex)
- Keyword type (TODO, FIXME, REVP, RPT, …) — restrict to one or more markers
- Scope: file path prefix, directory, or glob pattern

The `--grep` flag already exists on `comment list`; extend it (or add `--keyword` / `--scope`) to the top-level `list` command as well so in-code markers and tickets can both be narrowed down in one view.

## rpt/tdb: comment structure linting (like disco project)

Ability to enforce that in-code comment markers adhere to a declared format.
Possible approaches:
- Hardcode a set of structural rules into the tool (e.g. TODO must be `TODO(owner): text`, FIXME must include a ticket ref)
- Or allow `revparrot.toml` to declare a `format` regex per marker keyword

Open questions:
- Does this live in rpt (rule-based linting focus) or tdb (marker scanning focus)?
- How do violations surface — as `rpt check` output, or a separate `tdb lint` command?
- Reference: "disco" project for prior art on the structure-enforcement pattern.

## rpt: verbose mode for `rpt check` ✓ done

Default output is verbose blocks (rule + description, styled file path, code context,
message). Use `--oneline` for compact single-line output.

## rpt: output improvements for `rpt check` / `rpt diff`

Several output quality improvements to address:
- ✓ File paths are relative (to cwd or config root)
- ✓ Colour output: rule codes in bright red, file paths and line numbers styled;
  lipgloss respects NO_COLOR env and non-TTY automatically
- Remove redundant file path repetition when multiple violations are in one file
  (group by file, print path once as a header)
- Keyword in output (e.g. `REVP(use-pathlib)`) is redundant if rpt only scans one keyword;
  consider suppressing it — unless rules gain per-rule associated keywords (see keyword rename note),
  in which case showing the keyword becomes meaningful again
- ✓ Add explicit `--color` / `--colour` and `--no-color` / `--no-colour`
  flags (in addition to automatic TTY detection and the NO_COLOR env)

## rpt: restrict check to config-defined rules by default

Currently `rpt check` reports any `REVP(code)` annotation found in source files,
regardless of whether `code` matches a rule in `revparrot.toml`. This means:
- Violations appear with no title/description when the config isn't found or doesn't define the rule
- Running from a parent directory (above the codebase) surfaces violations the tool has no context for

Default behaviour should be: when a config is present, only report violations whose
code matches a defined (and enabled) rule. Annotations with unknown codes are silently
skipped, matching the principle that the config declares what the tool cares about.

Controls to relax this:
- `--all-codes` flag (or similar): report all REVP annotations regardless of config
- Config option `[revparrot] all_codes = true`: opt the whole project into the
  permissive mode
- When no config is found at all, current behaviour (report everything) can stay
  as a reasonable fallback for ad-hoc use

Open question: should unknown codes produce a warning rather than be silently skipped?

## rpt: `-n` flag for `rpt check`

Like `dfd` `-n`: show the total violation count but only render up to N blocks/lines.
Useful when there are many violations and you want to see a sample without overwhelming output.
Summary line should still show the full count, e.g. `Showing 5 of 23 violations.`

## rpt: `--statistics` flag for `rpt check`

Show a summary table of violations grouped by rule code, with counts.
Optionally include the rule description/title.
Example output:
  use-pathlib    9   Use pathlib for all path operations
  no-bare-exec   4   Avoid bare exec() calls
  total         13

Could be combined with other flags (e.g. `--statistics --rule use-pathlib` for a
per-file breakdown under that rule).
