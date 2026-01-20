; Source: github.com/camdencheek/tree-sitter-dockerfile @ v0.2.0
; Upstream: https://github.com/camdencheek/tree-sitter-dockerfile/blob/v0.2.0/queries/highlights.scm
;
; IMPORTANT: Upstream queries use "first match wins" but our highlighter uses
; "last match wins" (see MergeSpans). Queries may need reordering after fetching:
; general patterns (like @variable) should come BEFORE specific patterns (like @function).
;
; Check for LOCAL MODIFICATION comments below for any manual changes.

[
	"FROM"
	"AS"
	"RUN"
	"CMD"
	"LABEL"
	"EXPOSE"
	"ENV"
	"ADD"
	"COPY"
	"ENTRYPOINT"
	"VOLUME"
	"USER"
	"WORKDIR"
	"ARG"
	"ONBUILD"
	"STOPSIGNAL"
	"HEALTHCHECK"
	"SHELL"
	"MAINTAINER"
	"CROSS_BUILD"
	(heredoc_marker)
	(heredoc_end)
] @keyword

[
	":"
	"@"
] @operator

(comment) @comment


(image_spec
	(image_tag
		":" @punctuation.special)
	(image_digest
		"@" @punctuation.special))

[
	(double_quoted_string)
	(single_quoted_string)
	(json_string)
	(heredoc_line)
] @string

(expansion
  [
	"$"
	"{"
	"}"
  ] @punctuation.special
) @none

((variable) @constant
 (#match? @constant "^[A-Z][A-Z_0-9]*$"))


