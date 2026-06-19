# Plan

## rpt: rename REVP/NOREVP keywords to align with the binary name

Currently the annotation keyword REVP has no obvious connection to the `rpt` binary.
Consider renaming to RPT/NORPT (or similar). Decision points:
- Should the keyword match the binary exactly (rpt → RPT/NORPT)?
- Should rules be able to declare a custom associated keyword (e.g. rule `use-pathlib` → PATHLIB/NOPATHLIB)?
  If so, RPT/NORPT becomes the generic fallback and `rpt diff` output should show which keyword to use.
- Backward compat: if we rename, old REVP annotations in codebases become invisible — need a migration path or dual-scan period.

## rpt: promote `show` to a top-level subcommand

Currently `rpt diff --show [ref]` is a flag on the diff subcommand.
Should become `rpt show [ref]` for consistency with `dfd show`.
The diff subcommand then covers only multi-ref / working-tree diffs.
Update completions, usage strings, and help text accordingly.

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

## rpt: verbose mode for `rpt check`

Add a `-v` flag to `rpt check` that displays each violation in a rich block format,
similar to `dfd comment list -v`. The block would show:
- File + line (with surrounding context lines?)
- Rule code and full rule description
- The annotation message

Compact (default) output stays as-is (one line per violation).
See `tdb comment list -v` for the block style to mirror.

## rpt: output improvements for `rpt check` / `rpt diff`

Several output quality improvements to address:
- File paths should be relative (to cwd or config root), not absolute
- Remove redundant file path repetition when multiple violations are in one file
  (group by file, print path once as a header)
- Keyword in output (e.g. `REVP(use-pathlib)`) is redundant if rpt only scans one keyword;
  consider suppressing it — unless rules gain per-rule associated keywords (see keyword rename note),
  in which case showing the keyword becomes meaningful again
- Add colour output: rule codes, file paths, and line numbers in distinct colours;
  provide `--no-colour` / `NO_COLOR` env var to disable (follow NO_COLOR spec)
