; Go syntax highlighting queries
; Based on tree-sitter-go highlight queries

; Keywords
[
  "break" "case" "chan" "const" "continue" "default" "defer"
  "else" "fallthrough" "for" "func" "go" "goto" "if" "import"
  "interface" "map" "package" "range" "return" "select" "struct"
  "switch" "type" "var"
] @keyword

; Built-in types (these are identifiers, not node types)
((identifier) @type.builtin
 (#match? @type.builtin "^(bool|byte|complex64|complex128|error|float32|float64|int|int8|int16|int32|int64|rune|string|uint|uint8|uint16|uint32|uint64|uintptr)$"))

; Function definitions
(function_declaration
  name: (identifier) @function)

(method_declaration
  name: (field_identifier) @function.method)

; Function calls
(call_expression
  function: (identifier) @function)

(call_expression
  function: (selector_expression
    field: (field_identifier) @function.method))

; Built-in functions
(call_expression
  function: (identifier) @function.builtin
  (#match? @function.builtin "^(append|cap|close|complex|copy|delete|imag|len|make|new|panic|print|println|real|recover)$"))

; Types
(type_identifier) @type
(field_identifier) @property
(identifier) @variable

; Literals
[
  (nil)
  (true)
  (false)
] @constant.builtin

(int_literal) @number
(float_literal) @number
(rune_literal) @string
(interpreted_string_literal) @string
(raw_string_literal) @string

; Comments
(comment) @comment

; Operators
[
  "+"
  "-"
  "*"
  "/"
  "%"
  "<<"
  ">>"
  "&"
  "|"
  "^"
  "&^"
  "+="
  "-="
  "*="
  "/="
  "%="
  "<<="
  ">>="
  "&="
  "|="
  "^="
  "&^="
  "&&"
  "||"
  "=="
  "!="
  "<"
  "<="
  ">"
  ">="
  "="
  ":="
  "!"
  "++"
  "--"
  "..."
  "<-"
] @operator

; Punctuation
[
  "("
  ")"
  "["
  "]"
  "{"
  "}"
  ","
  ";"
  ":"
  "."
] @punctuation.delimiter