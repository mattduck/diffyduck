---
user-invocable: true
---

Audit review comments on the current branch to check they've been addressed.

Additional context from user: $ARGUMENTS

If the user specifies a time range (e.g. "last 2 days", "past 3 hours"), add
`--since <duration>` to the commands below. Format: 30m, 6h, 30d, 2w, 3M, 1y
(note: `m` is minutes, `M` is months).

1. Run `tdb list --json --store db --kind comment -n 0` to get all unresolved
   comments on the current branch as a JSON array (each row has `id`, `file`,
   `line`, `author`, `resolved`, `text`, and the full `body`).
2. If the array is empty, run
   `tdb list --json --store db --kind comment -n 15 --status resolved`
   to get the 15 most recently resolved comments and double-check them.
3. Delegate the analysis to a sub-agent: for each comment, have the agent read
   the current state of the referenced file and line, compare the code against
   what the comment was asking for, and classify each comment as:
   - Unresolved and still needs attention.
   - Resolved but the code doesn't appear to actually address the feedback.
   - Properly addressed (no action needed).
4. Present a summary: how many comments total, how many look good, and list only
   the flagged ones (unresolved, or resolved-but-not-addressed) with the comment
   ID, file, line, and a brief explanation of the concern.
