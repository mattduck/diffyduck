package main

import (
	"strings"

	"github.com/mattduck/diffyduck/pkg/highlight"
	"github.com/mattduck/diffyduck/pkg/ticketcli"
	"github.com/mattduck/diffyduck/pkg/ticketdb"
)

// contextHighlighter is dfd's tree-sitter-backed implementation of
// ticketcli.ContextHighlighter. It is what gives `dfd comment list -v` its
// syntax-colored code context. The tdb binary omits it (passes nil) and so stays
// free of the cgo tree-sitter dependency.
//
// The underlying highlighter is created lazily on first use and reused across
// comments, so non-block command paths never construct one.
type contextHighlighter struct {
	h *highlight.Highlighter
}

var _ ticketcli.ContextHighlighter = (*contextHighlighter)(nil)

func newContextHighlighter() *contextHighlighter {
	return &contextHighlighter{}
}

// Close releases the underlying tree-sitter highlighter, if one was created.
func (a *contextHighlighter) Close() {
	if a.h != nil {
		a.h.Close()
		a.h = nil
	}
}

// HighlightContext applies syntax highlighting to a comment's context lines
// (above + target + below), falling back to plain text when the language isn't
// supported.
func (a *contextHighlighter) HighlightContext(c *ticketdb.Comment) []string {
	if a.h == nil {
		a.h = highlight.New()
	}
	h := a.h

	allLines := make([]string, 0, len(c.Context.Above)+1+len(c.Context.Below))
	allLines = append(allLines, c.Context.Above...)
	allLines = append(allLines, c.Context.Line)
	allLines = append(allLines, c.Context.Below...)

	if len(allLines) == 0 {
		return allLines
	}

	// Build a single snippet for tree-sitter.
	snippet := strings.Join(allLines, "\n")
	content := []byte(snippet)

	spans, _ := h.Highlight(c.File, content)
	if len(spans) == 0 {
		return allLines
	}

	theme := h.Theme()
	result := make([]string, len(allLines))
	offset := 0
	for i, line := range allLines {
		lineBytes := []byte(line)
		lineStart := offset
		lineEnd := offset + len(lineBytes)

		lineSpans := highlight.SpansForLine(spans, lineStart, lineEnd)
		if len(lineSpans) == 0 {
			result[i] = line
		} else {
			var sb strings.Builder
			pos := 0
			for _, s := range lineSpans {
				if s.Start > pos {
					sb.Write(lineBytes[pos:s.Start])
				}
				style := theme.Style(s.Category)
				sb.WriteString(style.Render(string(lineBytes[s.Start:s.End])))
				pos = s.End
			}
			if pos < len(lineBytes) {
				sb.Write(lineBytes[pos:])
			}
			result[i] = sb.String()
		}

		offset = lineEnd + 1 // +1 for newline
	}

	return result
}
