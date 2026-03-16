---
user-invocable: true
---

Audit review comments on the current branch to check they've been addressed.

Additional context from user: $ARGUMENTS

If the user specifies a time range (e.g. "last 2 days", "past 3 hours"), add
`--since <duration>` to the commands below. Format: 1h, 6h, 1d, 2d, 1w, 2w, 1m, 1y.

1. Run `dfd comment list -n 0 --raw` to get all unresolved comments on the
   current branch.
2. If there are no unresolved comments, run `dfd comment list -n 15 --status resolved --raw`
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
