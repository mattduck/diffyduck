; Source: github.com/justinmk/tree-sitter-ini @ master
; Upstream: https://github.com/justinmk/tree-sitter-ini/blob/master/queries/highlights.scm
;
; IMPORTANT: Upstream queries use "first match wins" but our highlighter uses
; "last match wins" (see MergeSpans). Queries may need reordering after fetching:
; general patterns (like @variable) should come BEFORE specific patterns (like @function).
;
; Check for LOCAL MODIFICATION comments below for any manual changes.

(section_name
  (text) @type) ; consistency with toml

(comment) @comment @spell

[
  "["
  "]"
] @punctuation.bracket

"=" @operator

(setting
  (setting_name) @property)

; (setting_value) @none ; grammar does not support subtypes
