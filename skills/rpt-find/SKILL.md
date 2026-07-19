---
user-invocable: true
---

Find rule violations across the repo and record them — either as `RPT` code
annotations or as `tdb` tickets, whichever the user prefers — fanning the work
out across subagents (one batch per rule). This is the **find** stage of the
review flow — it only *records* violations; it never fixes them. Use the fix
skill afterwards.

User instructions: $ARGUMENTS

## What this does

For every active rule in `revparrot.toml`, it inspects the files currently in
that rule's scope and records each genuine violation. There are two recording
modes — both capture the same three fields (marker `RPT`, the rule's `type`, and
the rule's `code` as the scope):

- **annotations** (in-code): inserts `RPT <type>(<scope>): message` on a new line
  above the offending line, in the file's comment syntax. Minimal-diff, travels
  with the branch, fixed later by editing the line. `rpt check` validates them.
- **tickets** (git-state): creates one tdb ticket per violation via
  `tdb add <file>:<line> --prefix RPT --type <type> --scope <scope>`.
  No code churn, id-addressable, resolved later with `tdb resolve <id>`.

## Phase 0: Choose the recording mode

Decide **annotations vs tickets** before doing anything else:

- If the user's instructions name it ("as tickets", "annotate the code", "record
  as comments/tickets"), honour that.
- Otherwise **ask the user** which they want, presenting the two options above
  (annotations = in-code, minimal-diff; tickets = git-state, id-addressable), and
  wait for their answer.

Carry the chosen mode through Phases 2–3.

## Phase 1: Scope

1. Work out the config to use:
   - If the user named a `-config <path>` or a repo/path, honour it.
   - Otherwise auto-discover `revparrot.toml` from the current directory.
   - If the user scoped to specific paths ("just the models feature"), pass
     those paths through to `rpt ls`.
2. Run `rpt ls --json [-config <path>] [path...]` and parse the result. For
   each rule you get: `code`, `type`, `title`, `description`, optional `model`
   and `effort`, and `files` (every in-scope file, repo-relative).
3. Only process rules that appear in the output (disabled rules are already
   excluded). Report to the user the rules you'll run and the file count each,
   e.g. "2 rules: studio-pages-use-components (71 files), …".

## Phase 2: Find (fan-out)

**Spawn exactly one `Agent` per rule** — each agent gets that rule alone plus
its complete in-scope file list. (One-agent-per-rule is the current prototype
shape; it's slower on rules with many files, which is an accepted tradeoff for
now. Batching within a rule is a later optimisation.)

1. Launch the rule agents in parallel — put the `Agent` tool calls in a single
   message (one per active rule).
   - If the rule has a `model` field, pass it as the Agent's `model` override.
     (Per-call reasoning `effort` is **not** controllable from the Agent tool —
     that's what the Phase-3 workflow graduation is for. Note it and move on.)
   - Use `subagent_type: general-purpose`.
2. **Every agent's prompt must contain** the rule `code`, `type`, `title`, full
   `description`, and complete file list — plus the **mode-specific** recording
   instructions below. Both modes share the same *judgement* rules:

   > - Only flag genuine violations. Be conservative: a false positive costs
   >   more than a miss in this prototype.
   > - Respect the rule's stated exceptions. If a case is acceptable when marked
   >   `NORPT` (and the line already carries that `NORPT` for this scope), do not
   >   flag it.
   > - **Message — flag, do not fix:** state **what is wrong**, not how to fix it
   >   (e.g. "inline style object on the card wrapper"). Do NOT prescribe a
   >   solution, name replacement utilities/components, or suggest refactor steps
   >   — choosing the fix is a later stage's job. Keep it short and specific to
   >   the violating line.
   > - **Only count and report what YOU record in this run.** Exclude anything
   >   already recorded for this scope (see per-mode dedup below).

### If mode = annotations

Give the agent the exact annotation format and a filled-in example. The
**comment syntax depends on position**, and this matters in JSX:
- **JSX position** (the line above sits between JSX tags — the usual case for
  `.tsx`/`.jsx` UI rules): use the JSX block-comment form
  `{/* RPT <type>(<code>): message */}`. A bare `//` between JSX tags is **not** a
  comment — it renders as literal text. The scanner recognises the marker inside
  `{/* ... */}`.
- **Plain-JS position** (inside a function body, an object/array literal, a
  `.ts`/`.js`/`.go` file with no JSX around the line): use
  `// RPT <type>(<code>): message`. Use `#` for `.py`/`.sh`, `--` for `.sql`.

Agent instructions:

> Read each file. Where the code genuinely violates the rule, insert an
> annotation comment on **its own new line immediately above** the offending
> line, using the exact format `RPT <type>(<code>): <message>` with the type and
> code given, in the comment syntax valid for that position (see above).
>
> **Placement — keep the diff minimal:**
> - A **single new line directly above** the violating element/statement,
>   matching that line's indentation.
> - In JSX use the `{/* RPT ... */}` block form; only use `//` where the line
>   above is genuinely plain JS.
> - Do NOT reformat, re-indent, expand object literals, or otherwise touch
>   existing code. Add exactly one new line per violation and change nothing else.
> - Do NOT change any code behaviour — only add comment annotations.
> - **Dedup:** before annotating a line, check it does not already carry an `RPT`
>   or `NORPT` for this scope; if it does, leave it and do not count it.
>
> Return a structured summary of **only what you added this run**: per newly
> annotated file, its repo-relative path, and for each new annotation its line
> number + message. End with the count you added. If you added nothing, say so.

### If mode = tickets

Agent instructions:

> Read each file. Where the code genuinely violates the rule, record a ticket for
> the offending line — do NOT edit any files:
>
>     tdb add <file>:<line> --prefix RPT --type <type> --scope <code> --author Claude -m "<message>"
>
> using the exact `<type>` and `<code>` given.
> - **Dedup:** before you start, run once
>   `tdb list --store db --scope <code> --json` and skip any `<file>:<line>`
>   that already has a ticket for this scope, so re-runs don't duplicate.
> - Capture the id printed as `Created comment <id> on <file>:<line>`.
>
> Return a structured summary of **only the tickets you created this run**: per
> file, each line number + message + new ticket id. End with the count you
> created. If you created nothing, say so.

3. Collect the per-agent summaries. **These agent-reported counts — this run's
   additions only — are the source of truth for the final report**, because
   `tdb list` also shows items recorded on previous runs.
   - The tdb store serialises concurrent writes with CAS retries, so parallel
     `tdb add` calls across agents are safe (tickets mode).

## Phase 3: Validate & report

### If mode = annotations

1. Run `rpt check [-config <path>]` — the annotation-quality gate; it must pass
   clean. If it reports `rpt-syntax`, `rpt-unknown-scope`, or `rpt-type-mismatch`,
   an agent wrote a malformed annotation: read the offending lines, fix the
   format (do not change what was flagged), and re-run until clean.
2. **Cross-check** with `tdb list --store file --prefix RPT` (or `--scope <code>`
   per rule). If totals diverge from the agents' added-this-run counts by more
   than the pre-existing annotations, call it out.

### If mode = tickets

1. `rpt check` does **not** apply — it validates code annotations, and there are
   none. **Confirm** the tickets landed with
   `tdb list --store db --prefix RPT --json` (or `--scope <code>` per rule):
   its count should equal the agents' added-this-run totals plus any pre-existing
   tickets. If it diverges, an agent miscounted or a write failed — investigate.

### Both modes

Report to the user, driven by the **agents' added-this-run counts**:
- A per-rule table: rule, files scanned, violations **recorded this run**.
- The grand total recorded this run, the mode used, and the confirmation result
  (clean `rpt check` for annotations, matching `tdb list` count for tickets).
- A reminder that this only recorded violations; run the fix stage next
  (annotations are fixed by editing the line; tickets by `tdb resolve`).

## Notes

- `rpt ls` walks the working tree, so items from previous runs already show up in
  `tdb list`. For a fresh count, compare before/after or filter by this run's
  additions.
- Go `flag` stops at the first positional, so any flags must precede paths
  (`rpt ls --json src/pages`).
