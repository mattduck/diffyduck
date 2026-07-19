---
user-invocable: true
---

Find problems in code — either in a diff (code review) or against `rpt`
rules (compliance scan) — and record them. Recording uses `tdb` (see the
**`tdb` skill** for CLI mechanics); this skill only covers what to look for,
how to scope the search, and how to record findings.

User instructions: $ARGUMENTS

Runs interactively by default (can ask about scope, focus areas, ambiguous
calls). The one exception is the **review-fix** mode below, which explicitly
trades interactivity for running straight through to a fix.

## Mode: review a diff

Default when the user asks for a review with no rule/compliance framing
("review my changes", "review abc123", "review abc123..def456").

1. Determine scope from the request:
   - No ref → uncommitted changes: `git diff HEAD` (staged + unstaged).
   - Single ref → that commit: `git diff <ref>~1..<ref>` (or `git show <ref>`
     for fuller context). Anchor comments with `--commit <ref>`.
   - A range → `git diff <start>..<end>`. Anchor comments to the end ref.
2. Read the changed files for full context, not just the diff hunks.
3. Review for correctness, edge cases, clarity, bugs — or whatever focus the
   user specified.
4. For each real finding, `tdb add <file>:<line> --author Claude` (heredoc
   body). Every comment must request a change, flag a bug, or ask a
   question — never a comment that only describes or praises the code. Skip
   nitpicks unless asked for them.
5. Report a one-liner per comment created (or say the code looks good if
   none were left).

## Mode: rule/compliance scan

Trigger on intent ("find rule violations", "check compliance", "scan for
RPT issues") rather than any fixed phrase. If scope and recording target
aren't obvious from context, ask before running anything:

- **Scope** — current diff, or the whole repo / a subsection?
  - Diff: `rpt diff` (working tree vs HEAD; `-a` adds untracked, `--cached`
    for staged only, or pass ref(s) for `rpt diff <ref>` / `rpt diff <r1> <r2>`).
  - Whole repo or a subsection: `rpt ls [path...]` — walks the working tree
    (or just the given paths) independent of any diff.
  - A single commit: `rpt show [ref]` (defaults to HEAD).
- **Recording target** — inline `RPT` annotations in the code, or `tdb add`
  db comments?

Each of `rpt diff`/`ls`/`show` reports, per active rule, which in-scope files
it touches — that's your worklist. For batches of files/rules, delegate the
actual finding to subagents rather than churning through them serially:

- **Prefer the cheapest/fastest model that can do the job reliably.** Most
  `rpt` rules are mechanical pattern/style/naming checks with a clear
  yes/no — those are a good fit for a fast, cheap model. Only use a
  stronger model for rules that need real judgment (architectural or
  semantic calls), or if a fast-model pass is coming back noisy/wrong.
- Batch reasonably (e.g. one subagent per rule, or per file group) — don't
  spawn one subagent per file if the batch is small.

Record each genuine violation, one of two ways:

- **Annotation**: insert `RPT <type>(<scope>): message` as a new line at the
  violation. After annotating, run `rpt check` — it must stay clean; if it
  flags a malformed annotation you added, fix the format without changing
  what was flagged.
- **db comment**: `tdb add <file>:<line> --author Claude --prefix RPT --type
  <type> --scope <code>` (heredoc body).

This mode only records — it never fixes. Hand fixing off to the `work`
skill (or `review-fix` below).

## Mode: review-fix (review, then fix — non-interactive)

Only when the user explicitly wants review immediately followed by fixing,
unattended (e.g. "review and fix", "review this and clean it up").

1. Run the **review a diff** mode above, keeping the list of comment IDs
   created.
2. Hand those IDs to the **`work` skill**, invoked so it does not stop to
   ask questions (see `work`'s unattended behavior) — make the clear-cut
   fixes and resolve them per its author-aware rules; for anything
   non-obvious, make a best-effort fix if safe, otherwise leave it
   unresolved and record it, rather than blocking.
3. Output a summary table: `| ID | File:Line | Comment | Status |`, where
   Status is Resolved / Needs input (say what decision is needed) /
   Answered. Ask if the user wants to resolve any of the outstanding items.
