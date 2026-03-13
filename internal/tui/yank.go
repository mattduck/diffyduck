package tui

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/user/diffyduck/pkg/sidebyside"
)

// statusMessageMinDuration is the minimum time a status message is shown before
// it can be cleared by a keypress. After this period, the next keypress clears it.
const statusMessageMinDuration = 1 * time.Second

// handleYank is a context-sensitive "copy item" action:
//   - On a commit row: copies the full SHA
//   - On a comment or content row with a comment: copies the comment as a diff snippet
func (m Model) handleYank() (tea.Model, tea.Cmd) {
	rows := m.getRows()
	cursorPos := m.cursorLine()
	if cursorPos < 0 || cursorPos >= len(rows) {
		return m, nil
	}
	row := rows[cursorPos]

	// Commit header row: copy SHA
	if row.kind == RowKindCommitHeader {
		return m.yankCommitSHA(row.commitIndex)
	}

	// File header row: copy relative file path
	if row.kind == RowKindHeader {
		return m.yankFilePath(row.fileIndex)
	}

	// Comment / content rows: copy comment as diff snippet
	ck, found := m.findCommentForCursor()
	if !found {
		return m, nil
	}
	c := m.comments[ck]
	if c == nil || c.Text == "" {
		return m, nil
	}
	return m.yankComment(ck, c.Text)
}

// yankCommitSHA copies the full commit SHA to the clipboard.
func (m Model) yankCommitSHA(commitIndex int) (tea.Model, tea.Cmd) {
	if commitIndex < 0 || commitIndex >= len(m.commits) {
		return m, nil
	}
	sha := m.commits[commitIndex].Info.SHA
	if sha == "" {
		return m, nil
	}

	now := time.Now()
	if err := m.clipboard.Copy(sha); err != nil {
		m.statusMessage = fmt.Sprintf("Error: %v", err)
		m.statusMessageTime = now
		return m, m.clearStatusAfter(now)
	}

	m.statusMessage = fmt.Sprintf("Copied %s", m.commits[commitIndex].Info.ShortSHA())
	m.statusMessageTime = now
	return m, m.clearStatusAfter(now)
}

// yankFilePath copies the relative file path to the clipboard.
func (m Model) yankFilePath(fileIndex int) (tea.Model, tea.Cmd) {
	if fileIndex < 0 || fileIndex >= len(m.files) {
		return m, nil
	}
	fp := m.files[fileIndex]
	path := formatFilePath(fp.OldPath, fp.NewPath)
	if path == "" {
		return m, nil
	}

	now := time.Now()
	if err := m.clipboard.Copy(path); err != nil {
		m.statusMessage = fmt.Sprintf("Error: %v", err)
		m.statusMessageTime = now
		return m, m.clearStatusAfter(now)
	}

	m.statusMessage = fmt.Sprintf("Copied %s", path)
	m.statusMessageTime = now
	return m, m.clearStatusAfter(now)
}

// yankComment copies a comment as a unified diff snippet to the clipboard.
func (m Model) yankComment(ck commentKey, comment string) (tea.Model, tea.Cmd) {
	snippet := m.buildDiffSnippet(ck, comment)

	fileName := ""
	if ck.fileIndex >= 0 && ck.fileIndex < len(m.files) {
		fp := m.files[ck.fileIndex]
		fileName = formatFilePath(fp.OldPath, fp.NewPath)
	}

	now := time.Now()
	if err := m.clipboard.Copy(snippet); err != nil {
		m.statusMessage = fmt.Sprintf("Error: %v", err)
		m.statusMessageTime = now
		return m, m.clearStatusAfter(now)
	}

	m.statusMessage = fmt.Sprintf("Copied comment for %s", fileName)
	m.statusMessageTime = now
	return m, m.clearStatusAfter(now)
}

// handleVisualYank copies the rendered content of the visual selection to the clipboard.
// The output is the ANSI-stripped rendered text of each selected row, joined by newlines.
func (m Model) handleVisualYank() (tea.Model, tea.Cmd) {
	if !m.w().visualSelection.Active {
		return m, nil
	}

	// Compute selection range
	anchor := m.w().visualSelection.AnchorRow
	current := m.w().scroll
	selStart, selEnd := anchor, current
	if anchor > current {
		selStart, selEnd = current, anchor
	}

	// Get rows and clamp range
	rows := m.getRows()
	if selStart < 0 {
		selStart = 0
	}
	if selEnd >= len(rows) {
		selEnd = len(rows) - 1
	}
	if selStart > selEnd {
		return m, nil
	}

	// Compute layout widths (same formula as getVisibleRows)
	lineNumWidth := m.lineNumWidth()
	gutterOverhead := 1 + 1 + lineNumWidth + 1 + 4
	minRightWidth := 1 + 1 + lineNumWidth + 1 + 2
	targetLeftContent := 90
	if m.maxNewContentWidth < targetLeftContent {
		targetLeftContent = m.maxNewContentWidth
	}
	defaultHalf := (m.width - 3) / 2
	leftContentAt50 := defaultHalf - gutterOverhead
	var leftHalfWidth int
	if leftContentAt50 >= targetLeftContent {
		leftHalfWidth = defaultHalf
	} else {
		targetLeftWidth := gutterOverhead + targetLeftContent
		maxLeftWidth := m.width - 3 - minRightWidth
		leftHalfWidth = targetLeftWidth
		if leftHalfWidth > maxLeftWidth {
			leftHalfWidth = maxLeftWidth
		}
	}
	rightHalfWidth := m.width - 3 - leftHalfWidth
	rightContentArea := rightHalfWidth - lineNumWidth - 3 - 4
	hideRightTrailingGutter := rightContentArea <= 0

	// Render each selected row and strip ANSI
	lines := make([]string, 0, selEnd-selStart+1)
	for i := selStart; i <= selEnd; i++ {
		rendered := m.renderDisplayRow(rows[i], leftHalfWidth, rightHalfWidth, lineNumWidth, i, false, hideRightTrailingGutter)
		lines = append(lines, ansi.Strip(rendered))
	}

	content := strings.Join(lines, "\n")

	// Exit visual mode
	m.w().visualSelection.Active = false

	// Copy to clipboard
	now := time.Now()
	if err := m.clipboard.Copy(content); err != nil {
		m.statusMessage = fmt.Sprintf("Error: %v", err)
		m.statusMessageTime = now
		return m, m.clearStatusAfter(now)
	}

	lineCount := selEnd - selStart + 1
	m.statusMessage = fmt.Sprintf("Copied %d lines", lineCount)
	m.statusMessageTime = now
	return m, m.clearStatusAfter(now)
}

// handleYankAll copies all unresolved comments to the clipboard as a single unified diff patch.
// Comments are numbered globally (# MSG 1:, # MSG 2:, etc.) and nearby comments
// within the same file are merged into single hunks.
func (m Model) handleYankAll() (tea.Model, tea.Cmd) {
	if len(m.comments) == 0 {
		return m, nil
	}

	snippet, count := m.buildAllCommentsSnippet()
	if count == 0 {
		return m, nil
	}

	now := time.Now()
	if err := m.clipboard.Copy(snippet); err != nil {
		m.statusMessage = fmt.Sprintf("Error: %v", err)
		m.statusMessageTime = now
		return m, m.clearStatusAfter(now)
	}

	m.statusMessage = fmt.Sprintf("Copied %d unresolved comments", count)
	m.statusMessageTime = now
	return m, m.clearStatusAfter(now)
}

// commentWithKey pairs a comment key with its global message number for sorting.
type commentWithKey struct {
	key    commentKey
	msgNum int
}

// buildAllCommentsSnippet generates a unified diff patch containing all unresolved comments.
// Comments are sorted by file then line number, numbered globally, and nearby
// comments within the same file are merged into single hunks.
// Returns the snippet and the number of comments included.
func (m Model) buildAllCommentsSnippet() (string, int) {
	// Collect and sort all unresolved comments by (fileIndex, newLineNum)
	var sorted []commentWithKey
	for ck, c := range m.comments {
		if c == nil || c.Text == "" || c.Resolved {
			continue
		}
		sorted = append(sorted, commentWithKey{key: ck})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].key.fileIndex != sorted[j].key.fileIndex {
			return sorted[i].key.fileIndex < sorted[j].key.fileIndex
		}
		return sorted[i].key.newLineNum < sorted[j].key.newLineNum
	})

	// Assign global message numbers
	for i := range sorted {
		sorted[i].msgNum = i + 1
	}

	// Group by file
	type fileGroup struct {
		fileIndex int
		comments  []commentWithKey
	}
	var groups []fileGroup
	for _, cwk := range sorted {
		if len(groups) == 0 || groups[len(groups)-1].fileIndex != cwk.key.fileIndex {
			groups = append(groups, fileGroup{fileIndex: cwk.key.fileIndex})
		}
		groups[len(groups)-1].comments = append(groups[len(groups)-1].comments, cwk)
	}

	var sb strings.Builder

	for _, group := range groups {
		if group.fileIndex < 0 || group.fileIndex >= len(m.files) {
			continue
		}
		fp := m.files[group.fileIndex]

		// File headers
		sb.WriteString(fmt.Sprintf("--- %s\n", fp.OldPath))
		sb.WriteString(fmt.Sprintf("+++ %s\n", fp.NewPath))

		// Build merged hunks for this file
		m.writeFileCommentHunks(&sb, fp, group.comments)
	}

	return sb.String(), len(sorted)
}

// hunkRange represents a range of pair indices that form a hunk.
type hunkRange struct {
	startIdx int
	endIdx   int
	comments []commentWithKey // comments within this range
}

// writeFileCommentHunks writes merged hunks for all comments in a single file.
func (m Model) writeFileCommentHunks(sb *strings.Builder, fp sidebyside.FilePair, comments []commentWithKey) {
	const contextLines = 3

	// Build initial ranges for each comment
	var ranges []hunkRange
	for _, cwk := range comments {
		targetIdx := -1
		for i, pair := range fp.Pairs {
			if pair.New.Num == cwk.key.newLineNum {
				targetIdx = i
				break
			}
		}
		if targetIdx < 0 {
			continue
		}

		startIdx := targetIdx - contextLines
		if startIdx < 0 {
			startIdx = 0
		}
		ranges = append(ranges, hunkRange{
			startIdx: startIdx,
			endIdx:   targetIdx,
			comments: []commentWithKey{cwk},
		})
	}

	if len(ranges) == 0 {
		return
	}

	// Merge overlapping or adjacent ranges
	merged := []hunkRange{ranges[0]}
	for i := 1; i < len(ranges); i++ {
		prev := &merged[len(merged)-1]
		cur := ranges[i]
		// Merge if current starts within or adjacent to previous end (+1 for adjacency)
		if cur.startIdx <= prev.endIdx+1 {
			if cur.endIdx > prev.endIdx {
				prev.endIdx = cur.endIdx
			}
			prev.comments = append(prev.comments, cur.comments...)
		} else {
			merged = append(merged, cur)
		}
	}

	// Write each merged hunk
	for _, hr := range merged {
		oldStart, oldCount, newStart, newCount := m.calculateHunkHeader(fp.Pairs, hr.startIdx, hr.endIdx)
		sb.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount))

		// Build a set of commented line numbers for quick lookup
		commentsByLine := make(map[int]commentWithKey)
		for _, cwk := range hr.comments {
			commentsByLine[cwk.key.newLineNum] = cwk
		}

		for i := hr.startIdx; i <= hr.endIdx; i++ {
			pair := fp.Pairs[i]
			m.writeDiffLines(sb, pair)

			if cwk, ok := commentsByLine[pair.New.Num]; ok {
				c := m.comments[cwk.key]
				sb.WriteString(fmt.Sprintf("# MSG %d:\n", cwk.msgNum))
				for _, line := range strings.Split(c.Text, "\n") {
					sb.WriteString("# " + line + "\n")
				}
				sb.WriteString("#\n#\n")
			}
		}
	}
}

// AllCommentsSnippet returns the unified diff patch for all unresolved comments,
// or empty string if there are none. This is the exported version of
// buildAllCommentsSnippet, intended for printing after the TUI exits.
func (m Model) AllCommentsSnippet() string {
	if len(m.comments) == 0 {
		return ""
	}
	snippet, _ := m.buildAllCommentsSnippet()
	return snippet
}

// clearStatusAfter is a no-op kept for call-site compatibility. Status messages
// are now cleared on the next keypress after statusMessageMinDuration has elapsed.
func (m Model) clearStatusAfter(_ time.Time) tea.Cmd {
	return nil
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
//	 context line
//	+added line
//	# MSG 1:
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

	// Get context range (3 lines before, include the target line)
	contextLines := 3
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
			sb.WriteString("# MSG 1:\n")
			for _, line := range strings.Split(comment, "\n") {
				sb.WriteString("# " + line + "\n")
			}
			sb.WriteString("#\n#\n")
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

// Clipboard is the interface for copy/paste operations.
type Clipboard interface {
	Copy(text string) error
	Paste() (string, error)
}

// SystemClipboard uses the OS clipboard via pbcopy/pbpaste.
// TODO: Add support for Linux (xclip/xsel) and Windows (clip.exe)
type SystemClipboard struct{}

func (c *SystemClipboard) Copy(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func (c *SystemClipboard) Paste() (string, error) {
	cmd := exec.Command("pbpaste")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// MemoryClipboard is an in-memory clipboard for testing.
type MemoryClipboard struct {
	Content string
}

func (c *MemoryClipboard) Copy(text string) error {
	c.Content = text
	return nil
}

func (c *MemoryClipboard) Paste() (string, error) {
	return c.Content, nil
}
