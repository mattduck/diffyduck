---
user-invocable: true
---

General-purpose interface to `tdb`, the git-backed comment/issue store. Handles
plain record-keeping directly, and is the shared CLI reference the `review`
and `work` skills point to instead of re-explaining flags.

User instructions: $ARGUMENTS

## Vocabulary

`tdb list` merges two stores into one view:

- **db** — entries you write with `tdb add`/`edit`/`resolve`/`unresolve`.
  `--kind` distinguishes **comment** (has a `file:line`) from **issue**
  (standalone, no file:line).
- **file** — in-code markers (`RPT`, `TODO`, `FIXME`, …) found by scanning the
  working tree. Read-only from `tdb`'s side — created/removed by editing the
  code, not via `tdb add`.

Filters that apply across both stores: `--prefix`/`--exclude-prefix` (keyword,
e.g. `RPT`, `TODO`), `--type`, `--scope`, `--ticket`, `--file`, `--grep`.
db-only filters: `--status` (unresolved default/resolved/all), `--since`,
`--author`, `-b`/`--branch` (defaults to current branch), `--all-branches`.

Output/selection modifiers: `-v` (verbose blocks with code context), `--raw`
(db only), `-n[N]`, `--random` (shuffle, returns one unless `-n` given),
`--stats`/`--stats-group FIELD`, `--json`, `--exit-code` (CI gate).

Write commands (db only):

```
tdb add [file:line] [-m MSG | stdin] [--commit REF] [--author NAME]
                     [--prefix KW] [--type VALUE] [--scope CODE] [--ticket REF]
tdb edit <ID>            # opens $EDITOR; --resolved true|false to skip it
tdb resolve <ID>
tdb unresolve <ID>
```

## Direct add (no other skill needed)

For a plain "add a comment on X" / "add an issue for Y" / "note that Z"
request with no review or backlog-work implied:

1. Decide comment vs. issue: if the request names a specific file/line (or one
   is obvious from the current conversation), attach it — `tdb add
   <file>:<line>`. Otherwise it's standalone — `tdb add` with no path.
2. Write clear, concise text. Use `-m "text"` for a one-liner, a stdin
   heredoc for anything multi-line:
   ```
   tdb add <file>:<line> --author Claude <<'EOF'
   text here
   EOF
   ```
3. Tag with `--type`/`--scope`/`--ticket` only if the user gave you that
   context — don't invent classification that wasn't asked for.
4. Report the created ID back to the user.

## Listing / resolving in follow-up conversation

- List unresolved: `tdb list --json --kind comment` or `--kind issue`
  (scoped to current branch by default; `--all-branches` for every branch).
- Resolve: `tdb resolve <id>`. Only resolve entries the user asks you to
  resolve, or ones authored by "Claude" that you just fully addressed —
  never resolve human-authored entries without being told to.
- Comments can't be deleted, only edited (`tdb edit <id>`) or resolved.
