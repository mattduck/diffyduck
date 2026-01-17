package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/diffyduck/pkg/highlight"
)

// RequestHighlight returns a command that parses syntax highlighting for a file.
// This runs asynchronously to avoid blocking the UI.
func (m Model) RequestHighlight(fileIndex int) tea.Cmd {
	return func() tea.Msg {
		if fileIndex < 0 || fileIndex >= len(m.files) {
			return nil
		}

		fp := m.files[fileIndex]

		// Determine filename for language detection
		filename := fp.NewPath
		if filename == "/dev/null" {
			filename = fp.OldPath
		}

		// Check if highlighter supports this file type
		if m.highlighter == nil || !m.highlighter.SupportsFile(filename) {
			return nil
		}

		var oldSpans, newSpans []highlight.Span

		// Parse old content if available
		if len(fp.OldContent) > 0 {
			content := []byte(strings.Join(fp.OldContent, "\n"))
			spans, _ := m.highlighter.Highlight(filename, content)
			oldSpans = spans
		}

		// Parse new content if available
		if len(fp.NewContent) > 0 {
			content := []byte(strings.Join(fp.NewContent, "\n"))
			spans, _ := m.highlighter.Highlight(filename, content)
			newSpans = spans
		}

		// Convert to message format
		msg := HighlightReadyMsg{
			FileIndex: fileIndex,
			OldSpans:  convertSpans(oldSpans),
			NewSpans:  convertSpans(newSpans),
		}
		return msg
	}
}

// RequestHighlightAll returns a command that parses syntax highlighting for all files.
func (m Model) RequestHighlightAll() tea.Cmd {
	var cmds []tea.Cmd
	for i := range m.files {
		cmds = append(cmds, m.RequestHighlight(i))
	}
	return tea.Batch(cmds...)
}

// convertSpans converts highlight.Span to HighlightSpan (avoiding import in messages.go).
func convertSpans(spans []highlight.Span) []HighlightSpan {
	if len(spans) == 0 {
		return nil
	}
	result := make([]HighlightSpan, len(spans))
	for i, s := range spans {
		result[i] = HighlightSpan{
			Start:    s.Start,
			End:      s.End,
			Category: int(s.Category),
		}
	}
	return result
}

// storeHighlightSpans stores the spans from a HighlightReadyMsg into the model.
func (m *Model) storeHighlightSpans(msg HighlightReadyMsg) {
	m.highlightSpans[msg.FileIndex] = &FileHighlight{
		OldSpans: unconvertSpans(msg.OldSpans),
		NewSpans: unconvertSpans(msg.NewSpans),
	}
}

// unconvertSpans converts HighlightSpan back to highlight.Span.
func unconvertSpans(spans []HighlightSpan) []highlight.Span {
	if len(spans) == 0 {
		return nil
	}
	result := make([]highlight.Span, len(spans))
	for i, s := range spans {
		result[i] = highlight.Span{
			Start:    s.Start,
			End:      s.End,
			Category: highlight.Category(s.Category),
		}
	}
	return result
}

// getLineSpans returns syntax highlight spans for a specific line in a file.
// lineNum is 1-based line number, isOld indicates old vs new content side.
// The returned spans have byte offsets relative to the line start.
func (m Model) getLineSpans(fileIndex int, lineNum int, isOld bool) []highlight.Span {
	if lineNum <= 0 {
		return nil
	}

	fh, ok := m.highlightSpans[fileIndex]
	if !ok || fh == nil {
		return nil
	}

	fp := m.files[fileIndex]
	var content []string
	var allSpans []highlight.Span

	if isOld {
		content = fp.OldContent
		allSpans = fh.OldSpans
	} else {
		content = fp.NewContent
		allSpans = fh.NewSpans
	}

	if len(content) == 0 || len(allSpans) == 0 {
		return nil
	}

	// Calculate byte offsets for the target line
	// lineNum is 1-based, so line 1 starts at byte 0
	lineStart := 0
	for i := 0; i < lineNum-1 && i < len(content); i++ {
		lineStart += len(content[i]) + 1 // +1 for newline
	}

	if lineNum-1 >= len(content) {
		return nil
	}

	lineEnd := lineStart + len(content[lineNum-1])

	return highlight.SpansForLine(allSpans, lineStart, lineEnd)
}
