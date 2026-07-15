---
user-invocable: true
---

Add a note to track something for later.

User description: $ARGUMENTS

Based on the user's description, write a clear, concise note. If the
description is vague, infer the intent from the recent conversation context.

- If the note relates to a specific file and line, attach it there:
  ```
  tdb comment add <file>:<line> --author Claude <<'EOF'
  note text
  EOF
  ```
- Otherwise, create a standalone note:
  ```
  tdb comment add --author Claude <<'EOF'
  note text
  EOF
  ```

Use a stdin heredoc for multi-line notes, or `-m "text"` for short ones.

After adding the note, confirm what was created and show the ID.
