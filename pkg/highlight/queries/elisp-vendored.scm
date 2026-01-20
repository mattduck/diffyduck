; Source: github.com/Wilfred/tree-sitter-elisp @ main
; Upstream: https://github.com/Wilfred/tree-sitter-elisp/blob/main/queries/highlights.scm
;
; IMPORTANT: Upstream queries use "first match wins" but our highlighter uses
; "last match wins" (see MergeSpans). Queries may need reordering after fetching:
; general patterns (like @variable) should come BEFORE specific patterns (like @function).
;
; Check for LOCAL MODIFICATION comments below for any manual changes.

;; Special forms
[
  "and"
  "catch"
  "cond"
  "condition-case"
  "defconst"
  "defvar"
  "function"
  "if"
  "interactive"
  "lambda"
  "let"
  "let*"
  "or"
  "prog1"
  "prog2"
  "progn"
  "quote"
  "save-current-buffer"
  "save-excursion"
  "save-restriction"
  "setq"
  "setq-default"
  "unwind-protect"
  "while"
] @keyword

;; Function definitions
[
 "defun"
 "defsubst"
 ] @keyword
(function_definition name: (symbol) @function)
(function_definition parameters: (list (symbol) @variable.parameter))
(function_definition docstring: (string) @comment)

;; Highlight macro definitions the same way as function definitions.
"defmacro" @keyword
(macro_definition name: (symbol) @function)
(macro_definition parameters: (list (symbol) @variable.parameter))
(macro_definition docstring: (string) @comment)

(comment) @comment

(integer) @number
(float) @number
(char) @number

(string) @string

[
  "("
  ")"
  "#["
  "["
  "]"
] @punctuation.bracket

[
  "`"
  "#'"
  "'"
  ","
  ",@"
] @operator

;; Highlight nil and t as constants, unlike other symbols
[
  "nil"
  "t"
] @constant.builtin
