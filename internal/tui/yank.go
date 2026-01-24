package tui

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/diffyduck/pkg/sidebyside"
)

// statusMessageDuration is how long status messages are shown before auto-clearing.
const statusMessageDuration = 3 * time.Second

// handleYank copies the comment at the cursor position to the clipboard.
// The output is formatted as a unified diff snippet with the comment as # lines.
func (m Model) handleYank() (tea.Model, tea.Cmd) {
	// Find comment for current cursor position
	ck, found := m.findCommentForCursor()
	if !found {
		return m, nil // no comment to yank
	}

	comment := m.comments[ck]
	if comment == "" {
		return m, nil
	}

	// Build the diff snippet
	snippet := m.buildDiffSnippet(ck, comment)

	// Get file name for status message
	fileName := ""
	if ck.fileIndex >= 0 && ck.fileIndex < len(m.files) {
		fp := m.files[ck.fileIndex]
		fileName = formatFilePath(fp.OldPath, fp.NewPath)
	}

	// Copy to clipboard
	now := time.Now()
	if err := copyToClipboard(snippet); err != nil {
		m.statusMessage = fmt.Sprintf("Error: %v", err)
		m.statusMessageTime = now
		return m, m.clearStatusAfter(now)
	}

	m.statusMessage = fmt.Sprintf("Copied comment for %s", fileName)
	m.statusMessageTime = now
	return m, m.clearStatusAfter(now)
}

// clearStatusAfter returns a command that clears the status message after a delay.
func (m Model) clearStatusAfter(setTime time.Time) tea.Cmd {
	return tea.Tick(statusMessageDuration, func(t time.Time) tea.Msg {
		return ClearStatusMsg{SetTime: setTime}
	})
}

// findCommentForCursor returns the comment key for the current cursor position.
// Returns false if the cursor is not on a line with a comment.
func (m Model) findCommentForCursor() (commentKey, bool) {
	rows := m.getRows()
	cursorPos := m.cursorLine()
	if cursorPos < 0 || cursorPos >= len(rows) {
		return commentKey{}, false
	}

	row := rows[cursorPos]

	if row.kind == RowKindComment {
		// Cursor is on a comment row
		key := commentKey{fileIndex: row.fileIndex, newLineNum: row.commentLineNum}
		if _, ok := m.comments[key]; ok {
			return key, true
		}
	} else if row.kind == RowKindContent {
		// Cursor is on content, check if it has a comment
		if row.pair.New.Num > 0 {
			key := commentKey{fileIndex: row.fileIndex, newLineNum: row.pair.New.Num}
			if _, ok := m.comments[key]; ok {
				return key, true
			}
		}
	}

	return commentKey{}, false
}

// buildDiffSnippet generates a unified diff snippet with the comment.
// Format:
//
//	--- a/path/to/file
//	+++ b/path/to/file
//	@@ -X,Y +A,B @@
//	 context line
//	 context line
//	+added line
//	#
//	# comment text here
func (m Model) buildDiffSnippet(ck commentKey, comment string) string {
	fp := m.files[ck.fileIndex]

	// Find the pair index for the commented line
	targetPairIdx := -1
	for i, pair := range fp.Pairs {
		if pair.New.Num == ck.newLineNum {
			targetPairIdx = i
			break
		}
	}

	if targetPairIdx < 0 {
		// Fallback: just output comment with file info
		return fmt.Sprintf("# %s:%d\n#\n# %s",
			formatFilePath(fp.OldPath, fp.NewPath),
			ck.newLineNum,
			strings.ReplaceAll(comment, "\n", "\n# "))
	}

	// Get context range (2 lines before, include the target line)
	contextLines := 2
	startIdx := targetPairIdx - contextLines
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := targetPairIdx // inclusive, no lines after

	var sb strings.Builder

	// File headers
	sb.WriteString(fmt.Sprintf("--- %s\n", fp.OldPath))
	sb.WriteString(fmt.Sprintf("+++ %s\n", fp.NewPath))

	// Calculate hunk header values
	oldStart, oldCount, newStart, newCount := m.calculateHunkHeader(fp.Pairs, startIdx, endIdx)

	// Hunk header
	sb.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount))

	// Diff lines
	for i := startIdx; i <= endIdx; i++ {
		pair := fp.Pairs[i]
		m.writeDiffLines(&sb, pair)

		// If this is the commented line, add comment after
		if pair.New.Num == ck.newLineNum {
			sb.WriteString("#\n")
			for _, line := range strings.Split(comment, "\n") {
				sb.WriteString("# " + line + "\n")
			}
		}
	}

	return sb.String()
}

// calculateHunkHeader computes the old/new start and count for a hunk header.
func (m Model) calculateHunkHeader(pairs []sidebyside.LinePair, startIdx, endIdx int) (oldStart, oldCount, newStart, newCount int) {
	oldStart = 0
	newStart = 0
	oldCount = 0
	newCount = 0

	for i := startIdx; i <= endIdx; i++ {
		pair := pairs[i]

		// Track first line numbers
		if pair.Old.Num > 0 && oldStart == 0 {
			oldStart = pair.Old.Num
		}
		if pair.New.Num > 0 && newStart == 0 {
			newStart = pair.New.Num
		}

		// Count lines on each side
		switch {
		case pair.Old.Type == sidebyside.Context:
			// Context line appears on both sides
			oldCount++
			newCount++
		case pair.Old.Type == sidebyside.Removed:
			oldCount++
			if pair.New.Type == sidebyside.Added {
				newCount++
			}
		case pair.New.Type == sidebyside.Added:
			newCount++
		}
	}

	// Default start to 1 if we didn't find any lines
	if oldStart == 0 {
		oldStart = 1
	}
	if newStart == 0 {
		newStart = 1
	}

	return
}

// writeDiffLines writes the unified diff representation of a line pair.
func (m Model) writeDiffLines(sb *strings.Builder, pair sidebyside.LinePair) {
	switch {
	case pair.Old.Type == sidebyside.Context:
		// Context line (same on both sides)
		sb.WriteString(" " + pair.Old.Content + "\n")
	case pair.Old.Type == sidebyside.Removed && pair.New.Type == sidebyside.Added:
		// Changed line: output both
		sb.WriteString("-" + pair.Old.Content + "\n")
		sb.WriteString("+" + pair.New.Content + "\n")
	case pair.Old.Type == sidebyside.Removed:
		// Removed only
		sb.WriteString("-" + pair.Old.Content + "\n")
	case pair.New.Type == sidebyside.Added:
		// Added only
		sb.WriteString("+" + pair.New.Content + "\n")
	}
}

// copyToClipboard copies text to the system clipboard.
// TODO: Add support for Linux (xclip/xsel) and Windows (clip.exe)
func copyToClipboard(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// readFromClipboard reads text from the system clipboard.
// TODO: Add support for Linux (xclip/xsel) and Windows (clip.exe)
func readFromClipboard() (string, error) {
	cmd := exec.Command("pbpaste")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
