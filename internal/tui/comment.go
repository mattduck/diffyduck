package tui

import (
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
)

// handleCommentInput handles keypresses while in comment input mode.
func (m Model) handleCommentInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle bracketed paste (Cmd+V in terminal) - sanitize the entire pasted content
	if msg.Paste && len(msg.Runes) > 0 {
		text := sanitizePastedText(string(msg.Runes))
		if text != "" {
			before := m.commentInput[:m.commentCursor]
			after := m.commentInput[m.commentCursor:]
			m.commentInput = before + text + after
			m.commentCursor += len(text)
			m.commentEnsureCursorVisible()
			m.clampScroll() // main diff scroll may need adjustment due to prompt height change
		}
		return m, nil
	}

	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyCtrlG, tea.KeyEsc:
		// Cancel comment editing
		m.cancelComment()
		return m, nil

	case tea.KeyCtrlJ:
		// Ctrl+J to submit
		m.submitComment()
		return m, nil

	case tea.KeyEnter:
		// Enter inserts newline
		m.insertCommentRune('\n')
		m.commentEnsureCursorVisible()
		m.clampScroll() // main diff scroll may need adjustment due to prompt height change
		return m, nil

	case tea.KeyBackspace:
		m.commentDeleteBackward()
		m.commentEnsureCursorVisible()
		m.clampScroll() // main diff scroll may need adjustment due to prompt height change
		return m, nil

	case tea.KeyDelete:
		m.commentDeleteForward()
		m.commentEnsureCursorVisible()
		m.clampScroll() // main diff scroll may need adjustment due to prompt height change
		return m, nil

	case tea.KeyCtrlA:
		// Move to start of current line
		m.commentMoveLineStart()
		return m, nil

	case tea.KeyCtrlE:
		// Move to end of current line
		m.commentMoveLineEnd()
		return m, nil

	case tea.KeyCtrlF, tea.KeyRight:
		// Move forward one character
		m.commentMoveForward()
		return m, nil

	case tea.KeyCtrlB, tea.KeyLeft:
		// Move backward one character
		m.commentMoveBack()
		return m, nil

	case tea.KeyUp, tea.KeyCtrlP:
		// Move up one line
		m.commentMoveUp()
		return m, nil

	case tea.KeyDown, tea.KeyCtrlN:
		// Move down one line
		m.commentMoveDown()
		return m, nil

	case tea.KeyCtrlK:
		// Kill to end of line
		m.commentKillToEnd()
		m.commentEnsureCursorVisible()
		m.clampScroll() // main diff scroll may need adjustment due to prompt height change
		return m, nil

	case tea.KeyCtrlU:
		// Kill to beginning of line
		m.commentKillToStart()
		m.commentEnsureCursorVisible()
		m.clampScroll() // main diff scroll may need adjustment due to prompt height change
		return m, nil

	case tea.KeyCtrlV:
		// Paste from clipboard (explicit Ctrl+V) - read and sanitize
		text, err := readFromClipboard()
		if err != nil || text == "" {
			return m, nil
		}
		text = sanitizePastedText(text)
		// Insert at cursor position
		before := m.commentInput[:m.commentCursor]
		after := m.commentInput[m.commentCursor:]
		m.commentInput = before + text + after
		m.commentCursor += len(text)
		m.commentEnsureCursorVisible()
		m.clampScroll() // main diff scroll may need adjustment due to prompt height change
		return m, nil

	case tea.KeySpace:
		m.insertCommentRune(' ')
		return m, nil

	case tea.KeyRunes:
		for _, r := range msg.Runes {
			m.insertCommentRune(r)
		}
		return m, nil
	}

	return m, nil
}

// startComment enters comment mode for the current line.
// If there's an existing comment, loads it for editing.
func (m *Model) startComment() bool {
	cursorRow := m.cursorLine()

	// Get the display row at cursor
	rows := m.cachedRows
	if !m.rowsCacheValid {
		rows = m.buildRows()
	}
	if cursorRow < 0 || cursorRow >= len(rows) {
		return false
	}

	row := rows[cursorRow]
	if !m.canComment(row) {
		return false
	}

	// Set up comment key for this line
	m.commentKey = commentKey{
		fileIndex:  row.fileIndex,
		newLineNum: row.pair.New.Num,
	}

	// Load existing comment if any
	if existing, ok := m.comments[m.commentKey]; ok {
		m.commentInput = existing
		m.commentCursor = len(existing)
	} else {
		m.commentInput = ""
		m.commentCursor = 0
	}

	m.commentMode = true
	m.commentScroll = 0
	m.commentEnsureCursorVisible()
	return true
}

// submitComment saves the comment (or deletes if empty) and exits comment mode.
func (m *Model) submitComment() {
	text := strings.TrimSpace(m.commentInput)
	if text == "" {
		// Empty comment = delete
		delete(m.comments, m.commentKey)
	} else {
		m.comments[m.commentKey] = text
	}

	m.commentMode = false
	m.commentInput = ""
	m.commentCursor = 0
	m.commentScroll = 0

	// Invalidate row cache since comment rows changed
	m.rowsCacheValid = false
}

// cancelComment exits comment mode without saving.
func (m *Model) cancelComment() {
	m.commentMode = false
	m.commentInput = ""
	m.commentCursor = 0
	m.commentScroll = 0
}

// canComment returns true if the given row can have a comment attached.
// Any line with a valid line number on the new side is commentable.
func (m Model) canComment(row displayRow) bool {
	// Must be a content row
	if row.kind != RowKindContent {
		return false
	}

	// Must have a line number on the new side
	return row.pair.New.Num > 0
}

// insertCommentRune inserts a rune at the cursor position.
func (m *Model) insertCommentRune(r rune) {
	// Insert at cursor position
	before := m.commentInput[:m.commentCursor]
	after := m.commentInput[m.commentCursor:]
	m.commentInput = before + string(r) + after
	m.commentCursor += len(string(r))
}

// commentDeleteBackward deletes the character before the cursor.
func (m *Model) commentDeleteBackward() {
	if m.commentCursor == 0 {
		return
	}

	// Find the start of the previous rune
	before := m.commentInput[:m.commentCursor]
	runes := []rune(before)
	if len(runes) == 0 {
		return
	}
	newBefore := string(runes[:len(runes)-1])
	after := m.commentInput[m.commentCursor:]

	m.commentInput = newBefore + after
	m.commentCursor = len(newBefore)
}

// commentDeleteForward deletes the character after the cursor.
func (m *Model) commentDeleteForward() {
	if m.commentCursor >= len(m.commentInput) {
		return
	}

	// Find the end of the current rune
	after := m.commentInput[m.commentCursor:]
	runes := []rune(after)
	if len(runes) == 0 {
		return
	}
	newAfter := string(runes[1:])

	m.commentInput = m.commentInput[:m.commentCursor] + newAfter
}

// commentMoveForward moves the cursor forward one character.
func (m *Model) commentMoveForward() {
	if m.commentCursor >= len(m.commentInput) {
		return
	}

	after := m.commentInput[m.commentCursor:]
	runes := []rune(after)
	if len(runes) > 0 {
		m.commentCursor += len(string(runes[0]))
	}
}

// commentMoveBack moves the cursor backward one character.
func (m *Model) commentMoveBack() {
	if m.commentCursor == 0 {
		return
	}

	before := m.commentInput[:m.commentCursor]
	runes := []rune(before)
	if len(runes) > 0 {
		m.commentCursor = len(string(runes[:len(runes)-1]))
	}
}

// commentMoveLineStart moves the cursor to the start of the current line.
func (m *Model) commentMoveLineStart() {
	before := m.commentInput[:m.commentCursor]
	lastNewline := strings.LastIndex(before, "\n")
	if lastNewline == -1 {
		m.commentCursor = 0
	} else {
		m.commentCursor = lastNewline + 1
	}
}

// commentMoveLineEnd moves the cursor to the end of the current line.
func (m *Model) commentMoveLineEnd() {
	after := m.commentInput[m.commentCursor:]
	nextNewline := strings.Index(after, "\n")
	if nextNewline == -1 {
		m.commentCursor = len(m.commentInput)
	} else {
		m.commentCursor += nextNewline
	}
}

// commentKillToEnd deletes from cursor to end of line.
func (m *Model) commentKillToEnd() {
	after := m.commentInput[m.commentCursor:]
	nextNewline := strings.Index(after, "\n")
	if nextNewline == -1 {
		// Kill to end of input
		m.commentInput = m.commentInput[:m.commentCursor]
	} else {
		// Kill to newline (keep the newline)
		m.commentInput = m.commentInput[:m.commentCursor] + after[nextNewline:]
	}
}

// commentKillToStart deletes from cursor to beginning of line.
func (m *Model) commentKillToStart() {
	before := m.commentInput[:m.commentCursor]
	after := m.commentInput[m.commentCursor:]

	lastNewline := strings.LastIndex(before, "\n")
	if lastNewline == -1 {
		// On first line, kill from beginning
		m.commentInput = after
		m.commentCursor = 0
	} else {
		// Kill from after the newline to cursor
		m.commentInput = before[:lastNewline+1] + after
		m.commentCursor = lastNewline + 1
	}
}

// sanitizePastedText normalizes line endings and removes problematic characters.
func sanitizePastedText(text string) string {
	// First pass: normalize all line ending variants to \n
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\u2028", "\n") // Unicode Line Separator
	text = strings.ReplaceAll(text, "\u2029", "\n") // Unicode Paragraph Separator
	text = strings.ReplaceAll(text, "\x85", "\n")   // NEL (Next Line)

	// Second pass: filter out problematic characters
	var result strings.Builder
	result.Grow(len(text))
	for _, r := range text {
		// Skip ASCII control characters except \n and \t
		if r < 32 && r != '\n' && r != '\t' {
			continue
		}
		// Skip DEL
		if r == 127 {
			continue
		}
		// Skip zero-width and invisible Unicode characters
		if isProblematicUnicode(r) {
			continue
		}
		result.WriteRune(r)
	}

	// Strip trailing whitespace/newlines that can cause empty line issues
	// Use unicode.IsSpace to catch all Unicode whitespace variants (e.g., NO-BREAK SPACE U+00A0)
	return strings.TrimRightFunc(result.String(), unicode.IsSpace)
}

// isProblematicUnicode returns true for Unicode characters that can cause display issues.
func isProblematicUnicode(r rune) bool {
	switch {
	// Zero-width characters (invisible, can confuse cursor/width)
	case r == '\u200B': // Zero Width Space
		return true
	case r == '\u200C': // Zero Width Non-Joiner
		return true
	case r == '\u200D': // Zero Width Joiner
		return true
	case r == '\uFEFF': // Zero Width No-Break Space (BOM)
		return true
	case r == '\u2060': // Word Joiner
		return true
	}
	return false
}

// commentCursorLineIndex returns the 0-based line index where the cursor is.
func (m *Model) commentCursorLineIndex() int {
	before := m.commentInput[:m.commentCursor]
	return strings.Count(before, "\n")
}

// commentEnsureCursorVisible adjusts commentScroll to keep the cursor visible.
func (m *Model) commentEnsureCursorVisible() {
	cursorLine := m.commentCursorLineIndex()
	maxVisible := m.commentMaxVisibleLines()

	// If cursor is above visible area, scroll up
	if cursorLine < m.commentScroll {
		m.commentScroll = cursorLine
	}

	// If cursor is below visible area, scroll down
	if cursorLine >= m.commentScroll+maxVisible {
		m.commentScroll = cursorLine - maxVisible + 1
	}

	// Clamp scroll to valid range
	totalLines := strings.Count(m.commentInput, "\n") + 1
	maxScroll := totalLines - maxVisible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.commentScroll > maxScroll {
		m.commentScroll = maxScroll
	}
	if m.commentScroll < 0 {
		m.commentScroll = 0
	}
}

// commentMoveUp moves the cursor up one line, preserving column position.
func (m *Model) commentMoveUp() {
	before := m.commentInput[:m.commentCursor]

	// Find the start of the current line
	currentLineStart := strings.LastIndex(before, "\n")
	if currentLineStart == -1 {
		// Already on first line, can't go up
		return
	}

	// Column position within current line
	col := m.commentCursor - currentLineStart - 1

	// Find the start of the previous line
	prevLineStart := strings.LastIndex(before[:currentLineStart], "\n")
	if prevLineStart == -1 {
		prevLineStart = -1 // previous line starts at beginning
	}

	// Length of previous line
	prevLineLen := currentLineStart - prevLineStart - 1

	// Move cursor to same column on previous line (or end if shorter)
	if col > prevLineLen {
		col = prevLineLen
	}
	m.commentCursor = prevLineStart + 1 + col
	m.commentEnsureCursorVisible()
}

// commentMoveDown moves the cursor down one line, preserving column position.
func (m *Model) commentMoveDown() {
	before := m.commentInput[:m.commentCursor]
	after := m.commentInput[m.commentCursor:]

	// Find where current line starts
	currentLineStart := strings.LastIndex(before, "\n")
	if currentLineStart == -1 {
		currentLineStart = -1 // current line starts at beginning
	}

	// Column position within current line
	col := m.commentCursor - currentLineStart - 1

	// Find the next newline (end of current line)
	nextNewline := strings.Index(after, "\n")
	if nextNewline == -1 {
		// Already on last line, can't go down
		return
	}

	// Find the end of the next line (or end of input)
	nextLineStart := m.commentCursor + nextNewline + 1
	restAfterNextLine := m.commentInput[nextLineStart:]
	nextLineEnd := strings.Index(restAfterNextLine, "\n")
	var nextLineLen int
	if nextLineEnd == -1 {
		nextLineLen = len(restAfterNextLine)
	} else {
		nextLineLen = nextLineEnd
	}

	// Move cursor to same column on next line (or end if shorter)
	if col > nextLineLen {
		col = nextLineLen
	}
	m.commentCursor = nextLineStart + col
	m.commentEnsureCursorVisible()
}
