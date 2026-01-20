; Source: github.com/tree-sitter/tree-sitter-bash @ v0.25.1
; Upstream: https://github.com/tree-sitter/tree-sitter-bash/blob/v0.25.1/queries/highlights.scm
;
; IMPORTANT: Upstream queries use "first match wins" but our highlighter uses
; "last match wins" (see MergeSpans). Queries may need reordering after fetching:
; general patterns (like @variable) should come BEFORE specific patterns (like @function).
;
; Check for LOCAL MODIFICATION comments below for any manual changes.

[
  (string)
  (raw_string)
  (heredoc_body)
  (heredoc_start)
] @string

(command_name) @function

(variable_name) @property

[
  "case"
  "do"
  "done"
  "elif"
  "else"
  "esac"
  "export"
  "fi"
  "for"
  "function"
  "if"
  "in"
  "select"
  "then"
  "unset"
  "until"
  "while"
] @keyword

(comment) @comment

(function_definition name: (word) @function)

(file_descriptor) @number

[
  (command_substitution)
  (process_substitution)
  (expansion)
]@embedded

[
  "$"
  "&&"
  ">"
  ">>"
  "<"
  "|"
] @operator

(
  (command (_) @constant)
  (#match? @constant "^-")
)
