---
user-invocable: true
---

Fetch and address unresolved review comments from dfd.

Additional context from user: $ARGUMENTS

1. Run `dfd comment list -n 20 --raw` to get the 10 most recent unresolved
   comments on the current branch. We get this many because they might reference
   each other and give you additional context. Tell the user if there are no comments.
2. For each comment, read the referenced file and line to understand the context.
3. Address the feedback for the most recent comment only:
   - If the comment requests a code change, make the change.
   - If the comment asks a question, answer it.
   - If the comment is a statement or observation, acknowledge it and provide any relevant information.
4. After addressing a comment, decide whether to resolve it:
   - If the comment was authored by Claude (the author field will indicate this),
     resolve it automatically once the requested change is complete:
     `dfd comment resolve <id>`
   - However, if a Claude-authored comment is optional or suggestive (e.g. "you
     could...", "consider...", "might want to..."), ask the user whether they want
     it resolved instead of resolving automatically.
   - If the comment was authored by a human, do NOT resolve it — only the user
     should decide when human comments are resolved.
5. Ask the user if they want you to continue to the next comment, in which case try step 1 again.
