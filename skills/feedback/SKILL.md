---
user-invocable: true
---

Fetch and address unresolved review comments from tdb.

Additional context from user: $ARGUMENTS

1. Run `tdb list --json --source state --kind comment -n 20` to get the 20 most
   recent unresolved comments on the current branch as a JSON array. We get this
   many because they might reference each other and give you additional context.
   Each row carries `id`, `file`, `line`, `author`, `resolved`, `text` (one-line
   summary) and `body` (the full comment). Tell the user if the array is empty.
2. For each comment, read the referenced file and line to understand the context.
3. Address the feedback for the most recent comment only:
   - If the comment requests a code change, make the change.
   - If the comment asks a question, answer it.
   - If the comment is a statement or observation, acknowledge it and provide any relevant information.
4. After addressing a comment, decide whether to resolve it:
   - If the comment was authored by Claude (the `author` field will indicate this),
     resolve it automatically once the requested change is complete:
     `tdb comment resolve <id>`
   - However, if a Claude-authored comment is optional or suggestive (e.g. "you
     could...", "consider...", "might want to..."), ask the user whether they want
     it resolved instead of resolving automatically.
   - If the comment was authored by a human, do NOT resolve it — only the user
     should decide when human comments are resolved.
5. Ask the user if they want you to continue to the next comment, in which case try step 1 again.
