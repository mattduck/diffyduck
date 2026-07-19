---
user-invocable: true
---

Work the tdb backlog: pick an outstanding item (a rule violation, a marker, or a
review comment), make the change, and update its state. This is the generic
**consume-state** engine — `feedback` and `review-fix` delegate to it. It fixes
one item at a time by default; point it at a filter to narrow the backlog.

User instructions: $ARGUMENTS

## What this does

`tdb list` is the single inventory of outstanding work — it merges **tickets**
(git-state comments/notes, id-addressable) and **markers** (in-code `RPT`/`TODO`/
… annotations, no id). This skill reads that inventory, helps you choose
something to work on, works it, and records the result. It is aimed at
fix/refactor/perf-style items, not feature work.

The one field that matters is the **store**, because it determines how state is
updated:

- **ticket** (`store: "db"`, has an `id`) → fix the code, then resolve the
  ticket (`tdb resolve <id>`).
- **marker** (`store: "file"`, no `id`) → fix the code, then delete the
  annotation line. `rpt check` must stay clean afterwards.

## Phase 1: Choose what to work on

1. **Work out the target from the user's instructions.**
   - If they named a filter — a marker (`--prefix RPT`), a scope/rule
     (`--scope studio-a11y`), a type, `--kind comment`, a `--file`, a `--grep`,
     or `--store {db,file}` — carry those `tdb list` flags through.
   - If they named specific ticket ids or a `file:line`, work those directly.
   - If they gave nothing, do the **survey** below.

2. **Survey (no explicit target).** Run
   `tdb list --stats --json` for the **current branch** (main or feature —
   whatever is checked out; this is `tdb list`'s default). This returns a
   `{total, dimensions:[{field, counts:[{value, count}]}]}` breakdown across
   store/prefix/kind/type/scope — no need to aggregate rows yourself. Present it
   so the user can pick a category, e.g.:

   > On branch `feature/x` there are 31 open items:
   > - scope · studio-pages-use-components — 18
   > - scope · studio-tailwind-not-inline-styles — 9
   > - store · db — 4
   >
   > What would you like to work on?

   Use `--stats-group <field>` if you want a single-dimension count.

   Wait for the user to pick a category (or a specific item).

3. **Branch fallback.** If the survey finds **no items on the current branch**,
   tell the user and **ask whether to widen** — e.g. `--all-branches`, or a
   specific `--branch <name>`. Do not silently search other branches.
   - Note: markers always come from the working tree, so "no items" here means
     no matching *tickets* on this branch and no matching *markers* in the
     checked-out files.

## Phase 2: Select one item

Work **one item at a time** by default (a small same-rule batch is fine if the
user asks). Once a category/filter is settled:

1. Run `tdb list --json <filters>` and take the most relevant single row (most
   recent, or ask if it's ambiguous).
2. Note its `store`, `file`, `line`, `text`/`body`, and — for tickets — its
   `id` and `author`; for markers — its `prefix`/`type`/`scope`.
3. Read the referenced file around `line` to understand the full context.

## Phase 3: Work the item

Judge how clear-cut the change is:

- **Obvious / mechanical** (the item fully specifies the fix and there's one
  sensible way to do it) → make the change directly.
- **Non-obvious** (ambiguous, multiple reasonable approaches, or missing
  requirements) → this is normal development. **Ask the user the questions you
  need** to confirm requirements before editing, then make the change.

Keep the change scoped to the item. Don't fold in unrelated edits.

## Phase 4: Update state

### Ticket (has an id)

Resolve according to authorship (this is the same author-aware rule the
`feedback` flow uses):

- **Claude-authored** (the `author` field says so) and the fix is **clear-cut and
  complete** → resolve it: `tdb resolve <id>`.
- **Claude-authored but suggestive/optional** ("consider…", "you could…",
  "might want to…") → do **not** auto-resolve; leave it open and note it for the
  summary.
- **Human-authored** → do **not** auto-resolve; only the user decides. Report
  what you did and ask whether to resolve.

### Marker (no id)

Delete the annotation comment line (the `RPT …` / `TODO …` line) once the code is
fixed, leaving the corrected code. Then run `rpt check` — it must stay clean; if
it reports a broken annotation you left behind, fix the format (don't change what
was flagged) until clean.

## Phase 5: Commit (not baked in)

This skill does **not** commit by default — it edits code and updates tdb state,
then leaves committing to the user (`/md-commit`).

**If a commit is made and the item was a ticket, the commit message must
reference the ticket id.** Working one item at a time yields one commit per
ticket/comment. Markers carry no id, so nothing to reference.

## Phase 6: Continue

Report what you did (item, change, and resolution/removal). Then offer to work
the next item — loop back to Phase 2 (same filter) if the user wants to continue.

## Batch mode

When invoked with `--batch` (used by `review-fix`, which runs in a fork and
cannot ask questions):

- Work through the given set of ids/items in sequence, not one-at-a-time with
  prompts.
- Make the clear-cut fixes and auto-resolve them per the Phase-4 author rules.
- For any **non-obvious** item, do **not** block on questions — make a
  best-effort fix if safe, otherwise leave it unresolved and record it.
- End with a summary table of every item and its status (resolved / needs input /
  answered), instead of the interactive continue loop.

## Notes

- Branch scope only affects **tickets** (filtered by the branch they were filed
  on); **markers** are always the working tree. See the branch fallback in
  Phase 1.
- For a branch/worktree → PR → merge flow, prefer recording violations as
  **annotations** rather than tickets: the annotation (and its removal) rides
  with the code and merges atomically, so a fix can't be marked done on a branch
  that never lands. (Ticket state is a global ref, decoupled from the merge.)
- Go `flag` stops at the first positional, so any flags must precede paths.
