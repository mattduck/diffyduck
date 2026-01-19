; Source: nvim-treesitter @ master
; URL: https://raw.githubusercontent.com/nvim-treesitter/nvim-treesitter/master/queries/toml/highlights.scm
; Grammar: tree-sitter-toml v0.7.0
;
; This query uses "last match wins" semantics - general patterns should come
; before specific patterns so that specific patterns override them.
; See queries.go for more details.
;
; LOCAL MODIFICATION (re-apply after updating from nvim-treesitter):
; nvim-treesitter captures all bare_key as @property, making section headers
; like [server] and regular keys like "host = ..." the same color. We override
; this to capture section headers as @type for visual distinction.
; Original line was: (bare_key) @property

; Keys in key-value pairs
(pair
  (bare_key) @property)

; Table/section headers - styled as @type to differentiate from regular keys
(table
  (bare_key) @type)
(table
  (dotted_key
    (bare_key) @type))

; Array table headers [[like.this]]
(table_array_element
  (bare_key) @type)
(table_array_element
  (dotted_key
    (bare_key) @type))

[
  (string)
  (quoted_key)
] @string

(boolean) @boolean

(comment) @comment @spell

(escape_sequence) @string.escape

(integer) @number

(float) @number.float

[
  (local_date)
  (local_date_time)
  (local_time)
  (offset_date_time)
] @string.special

"=" @operator

[
  "."
  ","
] @punctuation.delimiter

[
  "["
  "]"
  "[["
  "]]"
  "{"
  "}"
] @punctuation.bracket
