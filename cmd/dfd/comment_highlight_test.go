package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattduck/diffyduck/pkg/ticketdb"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
)

// TestContextHighlighter_AddsColor verifies dfd's tree-sitter-backed adapter
// applies syntax highlighting (ANSI) to a supported language's context lines.
func TestContextHighlighter_AddsColor(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	ad := newContextHighlighter()
	defer ad.Close()

	c := &ticketdb.Comment{
		File: "test.go",
		Line: 3,
		Context: ticketdb.LineContext{
			Above: []string{"func foo() {", "    x := 1"},
			Line:  "    return x",
			Below: []string{"}"},
		},
	}

	lines := ad.HighlightContext(c)
	joined := strings.Join(lines, "\n")

	// Plain content is preserved...
	assert.Contains(t, stripANSIDfd(joined), "func foo() {")
	assert.Contains(t, stripANSIDfd(joined), "return x")
	// ...but the highlighted output carries ANSI codes.
	assert.Greater(t, len(joined), len(stripANSIDfd(joined)), "highlighting should add ANSI codes")
}

// TestContextHighlighter_UnsupportedLanguage verifies the adapter falls back to
// plain text for an unsupported file type.
func TestContextHighlighter_UnsupportedLanguage(t *testing.T) {
	ad := newContextHighlighter()
	defer ad.Close()

	c := &ticketdb.Comment{
		File:    "data.xyz",
		Line:    1,
		Context: ticketdb.LineContext{Line: "some content"},
	}

	lines := ad.HighlightContext(c)
	assert.Equal(t, []string{"some content"}, lines)
}

func stripANSIDfd(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
