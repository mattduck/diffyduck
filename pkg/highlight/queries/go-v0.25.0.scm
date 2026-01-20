; Source: github.com/tree-sitter/tree-sitter-go @ v0.25.0
; Upstream: https://github.com/tree-sitter/tree-sitter-go/blob/v0.25.0/queries/highlights.scm
;
; LOCAL MODIFICATION: Reordered for "last match wins" semantics.
; Our highlighter uses "last match wins" (see MergeSpans in spans.go), so general
; patterns must come BEFORE specific patterns. Upstream uses "first match wins".
; The identifiers section was moved before function calls/definitions.

; Identifiers (general - will be overridden by more specific patterns below)
(type_identifier) @type
(field_identifier) @property
(identifier) @variable

; Function calls (specific - override @variable above)
(call_expression
  function: (identifier) @function)

(call_expression
  function: (identifier) @function.builtin
  (#match? @function.builtin "^(append|cap|close|complex|copy|delete|imag|len|make|new|panic|print|println|real|recover)$"))

(call_expression
  function: (selector_expression
    field: (field_identifier) @function.method))

; Function definitions (specific - override @variable above)
(function_declaration
  name: (identifier) @function)

(method_declaration
  name: (field_identifier) @function.method)

; Operators
[
  "--"
  "-"
  "-="
  ":="
  "!"
  "!="
  "..."
  "*"
  "*"
  "*="
  "/"
  "/="
  "&"
  "&&"
  "&="
  "%"
  "%="
  "^"
  "^="
  "+"
  "++"
  "+="
  "<-"
  "<"
  "<<"
  "<<="
  "<="
  "="
  "=="
  ">"
  ">="
  ">>"
  ">>="
  "|"
  "|="
  "||"
  "~"
] @operator

; Keywords
[
  "break"
  "case"
  "chan"
  "const"
  "continue"
  "default"
  "defer"
  "else"
  "fallthrough"
  "for"
  "func"
  "go"
  "goto"
  "if"
  "import"
  "interface"
  "map"
  "package"
  "range"
  "return"
  "select"
  "struct"
  "switch"
  "type"
  "var"
] @keyword

; Literals
[
  (interpreted_string_literal)
  (raw_string_literal)
  (rune_literal)
] @string

(escape_sequence) @escape

[
  (int_literal)
  (float_literal)
  (imaginary_literal)
] @number

[
  (true)
  (false)
  (nil)
  (iota)
] @constant.builtin

(comment) @comment
