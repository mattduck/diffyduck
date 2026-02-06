package tui

import (
	"strings"
	"time"

	"github.com/user/diffyduck/pkg/comments"
	"github.com/user/diffyduck/pkg/sidebyside"
)

// cleanFilePath strips the a/ or b/ prefix from diff file paths.
// Diff format uses "a/path" for old and "b/path" for new, but we want
// clean paths for storage so comments are portable across diff views.
func cleanFilePath(path string) string {
	if strings.HasPrefix(path, "a/") {
		return strings.TrimPrefix(path, "a/")
	}
	if strings.HasPrefix(path, "b/") {
		return strings.TrimPrefix(path, "b/")
	}
	return path
}

// loadPersistedComments loads comments from the git store and matches them
// to current file positions. Returns the number of comments loaded.
func (m *Model) loadPersistedComments() int {
	if m.commentStore == nil {
		return 0
	}

	loaded := 0

	// Build a map of file path -> file index for quick lookup
	// Use clean paths (without a/ or b/ prefix) for portability
	filePathToIndex := make(map[string]int)
	for i, f := range m.files {
		filePathToIndex[cleanFilePath(f.NewPath)] = i
		// Also map old path in case file was renamed
		if f.OldPath != "" && f.OldPath != f.NewPath {
			filePathToIndex[cleanFilePath(f.OldPath)] = i
		}
	}

	// Load all comments from store
	allComments, err := m.commentStore.AllComments()
	if err != nil {
		// Silently fail - comments are non-critical
		return 0
	}

	for _, c := range allComments {
		// Find the file index
		fileIdx, ok := filePathToIndex[c.File]
		if !ok {
			// File not in current diff - comment is orphaned for this view
			continue
		}

		// Get the file content to match against
		file := m.files[fileIdx]
		fileLines := getFileLinesForMatching(file)
		if len(fileLines) == 0 {
			continue
		}

		// Find where the comment should appear
		result := comments.FindCommentPosition(c, fileLines)
		if !result.Found {
			// Comment's anchor doesn't match current content - orphaned
			continue
		}

		// Add to in-memory comments
		key := commentKey{
			fileIndex:  fileIdx,
			newLineNum: result.Line,
		}
		m.comments[key] = c.Text
		m.persistedCommentIDs[key] = c.ID
		loaded++
	}

	return loaded
}

// getFileLinesForMatching extracts file lines suitable for comment matching.
// It uses the new/left side line content from the file pairs.
func getFileLinesForMatching(file sidebyside.FilePair) []string {
	// Build a map of line number -> content from the pairs
	lineMap := make(map[int]string)
	maxLine := 0

	for _, pair := range file.Pairs {
		if pair.New.Num > 0 {
			lineMap[pair.New.Num] = pair.New.Content
			if pair.New.Num > maxLine {
				maxLine = pair.New.Num
			}
		}
	}

	if maxLine == 0 {
		return nil
	}

	// Convert to slice (may have gaps for deleted lines, but that's ok)
	lines := make([]string, maxLine)
	for i := 1; i <= maxLine; i++ {
		if content, ok := lineMap[i]; ok {
			lines[i-1] = content
		}
	}

	return lines
}

// persistComment saves a comment to the git store.
// Returns the comment ID on success.
func (m *Model) persistComment(key commentKey, text string) string {
	if m.commentStore == nil {
		return ""
	}

	// Get file info
	if key.fileIndex < 0 || key.fileIndex >= len(m.files) {
		return ""
	}
	file := m.files[key.fileIndex]

	// Get context for the line
	fileLines := getFileLinesForMatching(file)
	if key.newLineNum < 1 || key.newLineNum > len(fileLines) {
		return ""
	}

	ctx := comments.ExtractContextForLine(fileLines, key.newLineNum)
	now := time.Now()

	// Check if we're updating an existing persisted comment
	existingID := m.persistedCommentIDs[key]

	c := &comments.Comment{
		ID:      existingID, // Empty for new, existing ID for update
		Text:    text,
		File:    cleanFilePath(file.NewPath),
		Line:    key.newLineNum,
		Context: ctx,
		Anchor:  ctx.ComputeAnchor(),
		Updated: now,
	}

	// Set created time only for new comments
	if existingID == "" {
		c.Created = now
	}

	// Get current git state for metadata
	if m.git != nil {
		// Try to get HEAD SHA
		// Note: We don't have direct access to current commit SHA from here,
		// but we can store HEAD for context
	}

	// Get branch name if available
	if len(m.commits) > 0 && m.commits[0].Info.SHA != "" {
		c.CommitSHA = m.commits[0].Info.SHA
	}

	// Write to store
	id, err := m.commentStore.WriteComment(c)
	if err != nil {
		return ""
	}

	return id
}

// deletePersistedComment removes a comment from the git store.
func (m *Model) deletePersistedComment(key commentKey) {
	if m.commentStore == nil {
		return
	}

	id, ok := m.persistedCommentIDs[key]
	if !ok {
		return
	}

	// Delete from store (ignore errors - best effort)
	_ = m.commentStore.DeleteComment(id)
	delete(m.persistedCommentIDs, key)
}
