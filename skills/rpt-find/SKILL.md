---
user-invocable: true
---

Find rule violations across the repo and record them as `RPT` code
annotations, fanning the work out across subagents (one batch per rule).
This is the **find** stage of the review flow — it only *records* violations;
it never fixes them. Use `/dfd:rpt-fix` (or the fix skill) afterwards.

User instructions: $ARGUMENTS

## What this does

For every active rule in `revparrot.toml`, it inspects the files currently in
that rule's scope and inserts an annotation on each offending line:

```
RPT <type>(<scope>): short message explaining what to fix
```

`<type>` is the rule's `type` and `<scope>` is the rule's `code` — both must
match exactly or `rpt check` will flag the annotation as malformed.

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
2. **Each agent's prompt must contain**, verbatim where noted:
   - The rule `code`, `type`, `title`, and full `description`.
   - The exact annotation format and a filled-in example. The **comment syntax
     depends on the position**, and this matters in JSX:
     - **JSX position** (the line above sits between JSX tags — the usual case
       for `.tsx`/`.jsx` UI rules): use the JSX block-comment form
       `{/* RPT <type>(<code>): message */}`. A bare `//` between JSX tags is
       **not** a comment — it renders as literal text on the page. The scanner
       recognises the marker inside `{/* ... */}`.
     - **Plain-JS position** (inside a function body, an object/array literal, a
       `.ts`/`.js`/`.go` file with no JSX around the line): use `// RPT
       <type>(<code>): message`. Use `#` for `.py`/`.sh`, `--` for `.sql`.
   - The rule's complete file list.
   - These instructions for the agent:
     > Read each file. Where the code genuinely violates the rule described
     > above, insert an annotation comment on **its own new line immediately
     > above** the offending line, using the exact format
     > `RPT <type>(<code>): <message>` with the type and code given, in the
     > comment syntax valid for that position (see above).
     >
     > **Placement — keep the diff minimal:**
     > - The annotation goes on a **single new line directly above** the
     >   element/statement that violates the rule (matching that line's
     >   indentation).
     > - In JSX, use the `{/* RPT ... */}` block form — a bare `//` line between
     >   JSX tags renders as visible text and is wrong. Only use `//` where the
     >   line above is genuinely plain JS (not between JSX tags).
     > - Do NOT reformat, re-indent, expand object literals, or otherwise touch
     >   existing code to make room. Add exactly one new line per violation and
     >   change nothing else.
     >
     > **Message — flag, do not fix:** the message states **what is wrong**, not
     > how to fix it. Describe the violation only (e.g. "inline style object on
     > the card wrapper" or "hard-coded colour in a style attribute"). Do NOT
     > prescribe a solution, name replacement utilities/components, or suggest
     > refactor steps — choosing the fix is a later stage's job (likely a
     > different agent/model). Keep it short and specific to the violating line.
     >
     > - Only flag genuine violations. Be conservative: a false positive costs
     >   more than a miss in this prototype.
     > - Respect the rule's stated exceptions. If the rule says a case is
     >   acceptable when annotated `NORPT`, do not flag it.
     > - Do NOT change any code behaviour — only add comment annotations.
     > - **Only count and report annotations YOU add in this run.** Before
     >   annotating a line, check it does not already carry an `RPT` or `NORPT`
     >   for this scope — if it does, leave it and do NOT count it. Pre-existing
     >   annotations must be excluded from your totals.
     > - Preserve indentation and existing formatting.
     >
     > Return a structured summary of **only what you added this run**: for each
     > file you newly annotated, the repo-relative path, and for each new
     > annotation its line number + message. End with the count you added. If a
     > file had no new violations, don't list it. If you added nothing, say so.
3. Collect the per-agent summaries. **These agent-reported counts — additions
   only — are the source of truth for the final report**, because `tdb list`
   also shows annotations from previous runs.

## Phase 3: Validate & report

1. Run `rpt check [-config <path>]`. This is the annotation-quality gate — it
   must pass clean. If it reports `rpt-syntax`, `rpt-unknown-scope`, or
   `rpt-type-mismatch`, an agent wrote a malformed annotation: read the
   offending lines, fix the format (do not change what was flagged), and
   re-run until clean.
2. Report to the user, driven by the **agents' added-this-run counts**:
   - A per-rule table: rule, files scanned, annotations **added this run**.
   - The grand total added this run, and confirmation that `rpt check` is clean.
   - A reminder that this only recorded violations; run the fix stage next.
3. As a **cross-check only**, run `tdb list --source code --marker RPT` (or
   `--rule <code>` per rule) to show the full standing inventory. If its totals
   diverge from the agents' added-this-run counts by more than the
   pre-existing annotations, call that out — it means an agent miscounted or
   annotated something it shouldn't have.

## Notes

- `rpt ls` walks the working tree, so annotations from previous runs already in
  the tree will show up in `tdb list`. If the user asks for a fresh count,
  compare before/after or filter by what this run added.
- Go `flag` stops at the first positional, so any flags must precede paths
  (`rpt ls --json src/pages`).
