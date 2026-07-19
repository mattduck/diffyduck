---
user-invocable: true
---

Act on the `tdb` backlog: pick an outstanding item (a db comment/issue or a
code marker), fix it, and update its state. Also covers auditing whether
already-resolved items were actually fixed. See the **`tdb` skill** for CLI
mechanics; this skill covers what to do with the backlog.

User instructions: $ARGUMENTS

`tdb list` is the single inventory — it merges **db entries** (comments/
issues, id-addressable) and **markers** (in-code `RPT`/`TODO`/… annotations,
no id). The field that matters is the **store**, because it determines how
state is updated:

- **db** (has an id) → fix the code, then `tdb resolve <id>`.
- **file** (marker, no id) → fix the code, then delete the annotation line;
  `rpt check` must stay clean afterwards.

## Mode: work the backlog

1. **Work out the target.**
   - Named filter (a marker prefix, `--scope`, `--type`, `--kind comment`,
     `--file`, `--grep`, `--store {db,file}`) → carry those `tdb list` flags
     through.
   - Specific ids or a `file:line` → work those directly.
   - Asked for something random/arbitrary ("pick anything", "surprise me")
     → add `--random` to the `tdb list` call instead of surveying.
   - Nothing given → **survey**: `tdb list --stats --json` for the current
     branch (tdb's default). Present the `{total, dimensions}` breakdown so
     the user can pick a category, e.g.:

     > 31 open items on `feature/x`:
     > - scope · studio-pages-use-components — 18
     > - scope · studio-tailwind-not-inline-styles — 9
     > - store · db — 4
     >
     > What would you like to work on?

     Use `--stats-group <field>` for a single-dimension count. Wait for a
     pick.
   - No items found on the current branch → tell the user and ask whether
     to widen (`--all-branches` or a specific `--branch`); don't silently
     search elsewhere. Markers always come from the working tree, so "no
     items" there means no matching tickets *and* no matching markers in the
     checked-out files.

2. **Select one item.** Work one at a time by default (a small same-rule
   batch is fine if asked). `tdb list --json <filters>`, take the most
   relevant row (most recent, or ask if ambiguous). Note its store, file,
   line, text/body, and — for db entries — id and author; for markers —
   prefix/type/scope. Read the file around `line` for full context.

3. **Fix it.** Obvious/mechanical (fully specified, one sensible fix) → make
   the change directly. Non-obvious (ambiguous, multiple approaches, missing
   requirements) → ask what's needed before editing. Keep the change scoped
   to the item — don't fold in unrelated edits.

4. **Update state.**
   - db entry: resolve by authorship — Claude-authored and clear-cut/complete
     → `tdb resolve <id>`. Claude-authored but suggestive ("consider…",
     "might want to…") → leave open, note it in the summary. Human-authored
     → never auto-resolve; report what you did and ask.
   - Marker: delete the annotation line once the code is fixed, then run
     `rpt check` — it must stay clean; fix the annotation format (not what
     was flagged) if it isn't.

5. **Commit.** Not baked in — this skill edits code and updates tdb state,
   then leaves committing to the user (`/md-commit`). If a commit does get
   made for a db entry, its message must reference the entry's id (markers
   carry no id, so nothing to reference). Working one item at a time yields
   one commit per entry.

6. **Continue.** Report what you did, then offer to work the next item
   (loop to step 2 with the same filter).

### Running unattended

Some invocations can't stop to ask questions — chained from `review`'s
review-fix mode, or when the user explicitly says something like "fix all of
these, use your judgment, don't check in with me." In that case:

- Work through the target set in sequence rather than one-at-a-time with
  prompts.
- Make the clear-cut fixes and auto-resolve them per the state-update rules
  above.
- For anything non-obvious, don't block on a question — make a best-effort
  fix if it's safe, otherwise leave it unresolved and record it.
- End with a summary table of every item and its status (resolved / needs
  input / answered) instead of the interactive continue loop.

## Mode: sweep (audit resolved items)

Trigger on intent ("check these were actually addressed", "audit review
comments", "sweep"). Read-only — never edits code or changes tdb state.

1. `tdb list --json --store db --kind comment -n 0` → all unresolved
   comments on the current branch. If the user gave a time range ("last 2
   days"), add `--since <duration>` (30m, 6h, 30d, 2w, 3M, 1y).
2. If that's empty, also pull the 15 most recently resolved comments
   (`--status resolved -n 15`) to double-check those instead.
3. Delegate the comparison to a subagent per comment: read the current state
   of the referenced file/line, compare it against what the comment asked
   for, classify as:
   - Unresolved and still needs attention.
   - Resolved but the code doesn't appear to actually address it.
   - Properly addressed (no action needed).
   This needs real judgment (was the intent actually met, not just "is there
   a diff nearby"), so don't push it down to the cheapest model the way the
   `review` skill's mechanical rule-scan does — a standard-tier model is the
   right default here.
4. Report a summary count, then list only the flagged ones (unresolved, or
   resolved-but-not-addressed) with id, file, line, and a brief explanation.
