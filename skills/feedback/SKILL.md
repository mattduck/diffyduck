---
user-invocable: true
---

Fetch and address unresolved review comments from tdb.

Additional context from user: $ARGUMENTS

This is the `work` engine scoped to review comments. Invoke the **`work` skill**
with these presets and follow it:

- **Filter:** `--source state --kind comment` (git-state review comments only —
  not markers).
- **Branch:** the current branch (review comments are about this branch's
  changes). Read the 20 most recent unresolved comments
  (`tdb list --json --source state --kind comment -n 20`) so cross-referencing
  comments give each other context; work the most recent first.
- **Mode:** interactive, one comment at a time, with `work`'s author-aware
  resolve rules — Claude-authored + clear-cut → resolve automatically;
  Claude-authored but suggestive → ask; human-authored → never auto-resolve, the
  user decides.

Pass any user context above through to `work`. If the list is empty, tell the
user there's nothing to address.
