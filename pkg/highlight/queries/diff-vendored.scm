; Source: github.com/the-mikedavis/tree-sitter-diff @ main
; Upstream: https://github.com/the-mikedavis/tree-sitter-diff/blob/main/queries/highlights.scm
;
; IMPORTANT: Upstream queries use "first match wins" but our highlighter uses
; "last match wins" (see MergeSpans). Queries may need reordering after fetching:
; general patterns (like @variable) should come BEFORE specific patterns (like @function).
;
; Check for LOCAL MODIFICATION comments below for any manual changes.

(comment) @comment @spell

[
  (addition)
  (new_file)
] @diff.plus

[
  (deletion)
  (old_file)
] @diff.minus

(commit) @constant

(location) @attribute

(command
  "diff" @function
  (argument) @variable.parameter)

(filename) @string.special.path

(mode) @number

([
  ".."
  "+"
  "++"
  "+++"
  "++++"
  "-"
  "--"
  "---"
  "----"
] @punctuation.special
  (#set! priority 95))

[
  (binary_change)
  (similarity)
  (file_change)
] @label

(index
  "index" @keyword)

(similarity
  (score) @number
  "%" @number)
