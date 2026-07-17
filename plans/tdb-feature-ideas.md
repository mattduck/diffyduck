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

## 2. `--random` selection

A flag that returns a single random item to work on, respecting all active
filters (`--marker`, `--scope`, `--type`, `--source`, `--status`, …). Lets the
fix skill (and a human) grab "one thing to do" from a filtered backlog without
scanning the whole list.

- Likely `tdb list --random` (or `-1`/`--pick`): apply filters, return one item
  chosen at random. Pairs naturally with the fix skill's "work one item" loop.
- Consider determinism/seeding for tests (the codebase avoids `Math.random`
  patterns in some places; pick a testable RNG seam).

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

- `tdb --statistics` — per-rule/marker/kind counts. The fix skill's "what's
  available?" survey step wants this; until it exists the skill aggregates
  `tdb list --json` itself.
