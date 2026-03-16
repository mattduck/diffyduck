---
user-invocable: true
context: fork
---

Review code changes and leave comments using dfd.

User instructions: $ARGUMENTS

First, determine what to review based on the user instructions:

- **Default (no ref specified):** Review uncommitted changes (staged + unstaged).
  Run `git diff HEAD` to see what's changed since the last commit.
  Add comments via stdin heredoc:
  ```
  dfd comment add <file>:<line> --author Claude <<'EOF'
  comment text here
  EOF
  ```

- **Single commit (e.g. "review abc123"):** Review that commit's changes.
  Run `git diff <ref>~1..<ref>` (or `git show <ref>` for context).
  Anchor comments to the commit:
  ```
  dfd comment add <file>:<line> --ref <ref> --author Claude <<'EOF'
  comment text here
  EOF
  ```

- **Commit range (e.g. "review abc123..def456"):** Review the range.
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
   described above.
   - Pick the most relevant line for the comment.
   - Keep messages concise and actionable.
   - Every comment must request a change, flag a bug, or ask a question.
     Do NOT leave comments that merely describe, explain, or approve code
     (e.g. "this correctly handles X", "nice use of Y", "just noting that Z").
     If there's nothing to change, don't comment.
   - Don't leave trivial or nitpick comments unless the user asked for that.
4. Managing comments:
   - Comments cannot be deleted, but you can edit a comment's text:
     `dfd comment edit <id> -m "updated message"`
   - You can resolve comments authored by "Claude":
     `dfd comment resolve <id>`
   - NEVER resolve comments that have no author or a different author, unless
     the user explicitly tells you to.
5. After leaving all comments, report back a short summary: how many comments
   were left and a one-liner for each.

If the user asks to list or resolve comments in follow-up conversation:
- List unresolved: `dfd comment list --raw`
- Resolve a comment: `dfd comment resolve <id>`
- Only resolve comments when the user asks.
