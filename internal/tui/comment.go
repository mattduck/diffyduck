package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// handleCommentInput handles keypresses while in comment input mode.
func (m Model) handleCommentInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyCtrlG:
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
		return m, nil

	case tea.KeyBackspace:
		m.commentDeleteBackward()
		return m, nil

	case tea.KeyDelete:
		m.commentDeleteForward()
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

	// Invalidate row cache since comment rows changed
	m.rowsCacheValid = false
}

// cancelComment exits comment mode without saving.
func (m *Model) cancelComment() {
	m.commentMode = false
	m.commentInput = ""
	m.commentCursor = 0
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
}
