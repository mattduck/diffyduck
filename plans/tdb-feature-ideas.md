# tdb / review-flow feature ideas

Loosely-held feature ideas surfaced while designing the fix stage of the review
flow (see `plans/review-flow.md`). Not scheduled — a parking lot.

## 1. Comments/log against a ticket

Ability to leave a "comment" *against an existing ticket* — a running log or
reply thread on a work item, distinct from the current top-level `comment` kind
(which is itself a ticket). Use case in the fix flow: the fixer records progress
or blockers ("attempted X, blocked by Y", "partial fix — needs a follow-up")
against a ticket **without resolving it**, so the state carries a history rather
than just open/resolved.

- **Naming clash to resolve:** the existing `comment` is a top-level review
  ticket; this new thing is a *log entry / reply on* a ticket. Needs a
  non-colliding name (e.g. `log`, `reply`, `annotate`, `note-on`), and a clear
  data model (child entries under a ticket id vs a free-form appended body).
- Surfaces if built: `pkg/ticketdb` (data model + serialization),
  `pkg/ticketcli` (subcommand + rendering), `tdb list --json` (expose the log),
  completions.

## 2. `--random` selection — DONE

Implemented as `tdb list --random`: shuffles the filtered rows and returns one,
or `-n N` for N random items. Modelled as an ordering override (not a `--sort`
framework — none exists yet; that's the generalisation if other sort methods are
ever wanted). Routes through the merged list path like `--json`, and rejects
`-v`/`--raw`/ID lookup. Determinism handled via a `randShuffle` package seam that
tests override; production uses the auto-seeded global `math/rand` (Go 1.20+).
See `selectRows` in `pkg/ticketcli/list.go`.

## 3. Keep ticket lifecycle aligned with the git branch / merge

Today all tickets live in one global ref (`refs/dfd/comments`); the `# BRANCH:`
field only records the branch you were *on when the ticket was created*, and
`tdb list` merely filters on it (default: current branch). Ticket **state is
fully decoupled from whether the code change merges**. Two failure modes:

- **Resolved-but-never-merged (the lost-work case):** you branch off `main`, fix
  a violation, `tdb comment resolve`, but the branch is abandoned / PR rejected.
  The code fix disappears with the branch; the "resolved" mark survives in the
  global ref. The violation is still in `main` but the ticket says done.
- **Invisible-on-branch:** tickets created on `main` don't show on a feature
  branch by default (their `# BRANCH:` ≠ the current branch), so a
  branch-to-fix flow can't see the backlog it was created to work.

Ideas to explore:
- Record the **resolving commit/branch** on the ticket (extend the existing
  `# BRANCH_HEAD:` / `--ref` / `commit` machinery), and add a reconcile command
  that flags "resolved on branch X, but X isn't merged into <base>" → reopen or
  warn.
- Treat a ticket as truly closed only once its resolving change lands on the
  base branch; "promote" resolutions on merge, revert them if the branch dies.
- A way to **re-anchor / follow** tickets across branches (e.g. show tickets
  from the current branch *and* its base) so the backlog isn't stranded.

## Related (already noted in review-flow.md)

- `tdb list --stats` — DONE. Multi-dimension counts (source/marker/kind/type/
  scope) with `--stats-group FIELD` to collapse to one; `--json` for machines.
  The `work` survey step now reads `tdb list --stats --json` instead of
  hand-aggregating. (`rpt check --statistics` was renamed to `--stats` for
  consistency.) See `renderStats` in `pkg/ticketcli/list.go`.
