package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/diffyduck/pkg/sidebyside"
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
// Only lines with Added type on the new side are commentable.
func (m Model) canComment(row displayRow) bool {
	// Must be a content row
	if row.kind != RowKindContent {
		return false
	}

	// Must have a line number on the new side
	if row.pair.New.Num <= 0 {
		return false
	}

	// Line must be Added (includes pure additions and the new side of changed lines)
	return row.pair.New.Type == sidebyside.Added
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
