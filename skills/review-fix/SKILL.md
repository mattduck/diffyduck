---
user-invocable: true
context: fork
---

Review code changes, then address and resolve the review comments. Combines
the review and feedback skills into a single pass.

User instructions: $ARGUMENTS

## Phase 1: Review

Determine what to review based on the user instructions:

- **Default (no ref specified):** Review uncommitted changes (staged + unstaged).
  Run `git diff HEAD` to see what's changed since the last commit.
  Add comments via stdin heredoc:
  ```
  dfd comment add <file>:<line> --author Claude <<'EOF'
  comment text here
  EOF
  ```

- **Single commit (e.g. "review-fix abc123"):** Review that commit's changes.
  Run `git diff <ref>~1..<ref>` (or `git show <ref>` for context).
  Anchor comments to the commit:
  ```
  dfd comment add <file>:<line> --ref <ref> --author Claude <<'EOF'
  comment text here
  EOF
  ```

- **Commit range (e.g. "review-fix abc123..def456"):** Review the range.
  Run `git diff <start>..<end>` to see all changes.
  Anchor comments to the end ref:
  ```
  dfd comment add <file>:<line> --ref <end> --author Claude <<'EOF'
  comment text here
  EOF
  ```

Then:

1. Read the changed files to understand the full context around each change.
2. Conduct a code review. If the user provided instructions above, follow them
   (e.g. focus areas, review style). Otherwise, do a general review covering
   correctness, edge cases, clarity, and potential bugs.
3. For each piece of feedback, leave a comment using `dfd comment add` as
   described above. **Capture the comment ID from the output** (printed as
   `Created comment <id> on <file>:<line>`). Keep a list of all IDs created
   during this review.
   - Pick the most relevant line for the comment.
   - Keep messages concise and actionable.
   - Every comment must request a change, flag a bug, or ask a question.
     Do NOT leave comments that merely describe, explain, or approve code
     (e.g. "this correctly handles X", "nice use of Y", "just noting that Z").
     If there's nothing to change, don't comment.
   - Don't leave trivial or nitpick comments unless the user asked for that.
4. After leaving all comments, report how many were left with a one-liner for
   each, then proceed to Phase 2.

If no comments were left, skip Phase 2 and tell the user the code looks good.

## Phase 2: Fix

Work through **only the comments created in Phase 1** (using the captured IDs),
ignoring any other pre-existing comments.

For each comment:

1. Read the referenced file and line to understand the context.
2. Address the feedback:
   - If the comment requests a code change, make the change.
   - If the comment asks a question, answer it inline (edit the comment with
     the answer using `dfd comment edit <id> -m "answer"`).
3. After addressing the comment, decide whether to resolve it:
   - If the fix is clear-cut and complete, resolve it:
     `dfd comment resolve <id>`
   - If the comment is optional or suggestive (e.g. "you could...",
     "consider...", "might want to..."), do NOT resolve it yet — leave it
     for the summary table.

## Phase 3: Summary

Output a markdown table summarising every comment from this review:

```
| ID | File:Line | Comment | Status |
|----|-----------|---------|--------|
```

Status values:
- **Resolved** — the fix was made and the comment was resolved.
- **Needs input** — the comment is optional/suggestive; briefly say what
  decision the user needs to make.
- **Answered** — a question was answered (include the short answer).

Ask the user if they want to resolve any of the outstanding items.
