; Source: github.com/tree-sitter-grammars/tree-sitter-markdown @ v0.5.2
; Upstream: https://github.com/tree-sitter-grammars/tree-sitter-markdown/blob/v0.5.2/tree-sitter-markdown/queries/highlights.scm
;
; IMPORTANT: Upstream queries use "first match wins" but our highlighter uses
; "last match wins" (see MergeSpans). Queries may need reordering after fetching:
; general patterns (like @variable) should come BEFORE specific patterns (like @function).
;
; Check for LOCAL MODIFICATION comments below for any manual changes.

;From nvim-treesitter/nvim-treesitter
(atx_heading
  (inline) @text.title)

(setext_heading
  (paragraph) @text.title)

; LOCAL MODIFICATION: Use @keyword for markers to make them more visible
[
  (atx_h1_marker)
  (atx_h2_marker)
  (atx_h3_marker)
  (atx_h4_marker)
  (atx_h5_marker)
  (atx_h6_marker)
  (setext_h1_underline)
  (setext_h2_underline)
] @keyword

[
  (link_title)
  (indented_code_block)
  (fenced_code_block)
] @string

(fenced_code_block_delimiter) @keyword

(code_fence_content) @none

(link_destination) @string

(link_label) @constant

[
  (list_marker_plus)
  (list_marker_minus)
  (list_marker_star)
  (list_marker_dot)
  (list_marker_parenthesis)
  (thematic_break)
] @keyword

[
  (block_continuation)
  (block_quote_marker)
] @keyword

(backslash_escape) @string.escape
