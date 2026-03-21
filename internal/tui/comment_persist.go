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

// loadAllStoreComments fetches every comment from the store once and caches
// them for the top-bar count. Call recomputeCommentCounts after this (and
// after any mutation or filter change) to update the cached totals.
// Requires loadCommentIndex to have been called first (reuses the cached index
// to avoid a redundant git call).
func (m *Model) loadAllStoreComments() {
	if m.commentStore == nil || m.commentIndex == nil {
		return
	}
	if m.allStoreComments != nil {
		return // already loaded
	}
	ids := m.commentIndex.All()
	if len(ids) == 0 {
		m.allStoreComments = []*comments.Comment{}
		m.recomputeCommentCounts()
		return
	}
	all, err := m.commentStore.ReadCommentsBatch(ids)
	if err != nil {
		m.allStoreComments = []*comments.Comment{} // don't retry on error
		return
	}
	m.allStoreComments = all
	m.recomputeCommentCounts()
}

// recomputeCommentCounts recalculates cachedUnresolved and cachedResolved
// from allStoreComments, respecting the current branch filter.
func (m *Model) recomputeCommentCounts() {
	var unresolved, resolved int
	for _, c := range m.allStoreComments {
		if c.IsStandalone() {
			continue
		}
		if !m.isCommentIncluded(c) {
			continue
		}
		if c.Resolved {
			resolved++
		} else {
			unresolved++
		}
	}
	m.cachedUnresolved = unresolved
	m.cachedResolved = resolved
}

// upsertStoreComment updates or appends a comment in allStoreComments.
func (m *Model) upsertStoreComment(c *comments.Comment) {
	for i, sc := range m.allStoreComments {
		if sc.ID == c.ID {
			m.allStoreComments[i] = c
			return
		}
	}
	m.allStoreComments = append(m.allStoreComments, c)
}

// removeStoreComment removes a comment from allStoreComments by ID.
func (m *Model) removeStoreComment(id string) {
	for i, sc := range m.allStoreComments {
		if sc.ID == id {
			m.allStoreComments = append(m.allStoreComments[:i], m.allStoreComments[i+1:]...)
			return
		}
	}
}

// loadCommentIndex loads the comment index from the git store.
// The index is cheap (one git cat-file call for a small text blob) and maps
// file paths to comment IDs. It's loaded once and kept in memory.
func (m *Model) loadCommentIndex() {
	if m.commentStore == nil {
		return
	}

	// Cache the current branch for display-time comment filtering.
	// Set unconditionally (before the index guard) so the branch filter
	// works even when the index hasn't been created yet.
	if m.currentBranch == "" {
		if branch, err := m.commentStore.CurrentBranch(); err == nil && branch != "" {
			m.currentBranch = branch
		}
	}

	if m.commentIndex != nil {
		return
	}

	// First load: warn if detached HEAD means branch filtering is inactive.
	if m.currentBranch == "" {
		m.statusMessage = "Comments: showing all branches (detached HEAD)"
		m.statusMessageTime = time.Now()
	}
	idx, err := m.commentStore.ReadIndex()
	if err != nil {
		return
	}
	m.commentIndex = idx
}

// matchCommentsForFiles batch-fetches and matches persisted comments for the
// given file range. Files with no Pairs (skeletons) are silently skipped.
// Uses the cached index to determine which comments are relevant, and
// loadedCommentIDs to avoid re-fetching on fold/unfold cycles.
// Returns the number of newly matched comments.
func (m *Model) matchCommentsForFiles(startIdx, endIdx int) int {
	if m.commentStore == nil || m.commentIndex == nil {
		return 0
	}

	// Collect comment IDs we need, grouped by file index
	type fileMapping struct {
		fileIdx int
		ids     []string
	}
	var mappings []fileMapping
	var idsToFetch []string

	for i := startIdx; i < endIdx && i < len(m.files); i++ {
		f := m.files[i]
		if len(f.Pairs) == 0 {
			continue
		}

		// Check both new and old paths against the index
		paths := []string{cleanFilePath(f.NewPath)}
		if f.OldPath != "" && f.OldPath != f.NewPath {
			paths = append(paths, cleanFilePath(f.OldPath))
		}

		var relevantIDs []string
		seen := make(map[string]bool)
		for _, path := range paths {
			for _, id := range m.commentIndex.Get(path) {
				if !seen[id] && !m.loadedCommentIDs[id] {
					seen[id] = true
					relevantIDs = append(relevantIDs, id)
				}
			}
		}

		if len(relevantIDs) > 0 {
			idsToFetch = append(idsToFetch, relevantIDs...)
			mappings = append(mappings, fileMapping{fileIdx: i, ids: relevantIDs})
		}
	}

	if len(idsToFetch) == 0 {
		return 0
	}

	// Batch-fetch all needed comments in one git call
	fetched, err := m.commentStore.ReadCommentsBatch(idsToFetch)
	if err != nil {
		return 0
	}

	// Build lookup by ID and mark all as loaded
	commentByID := make(map[string]*comments.Comment, len(fetched))
	for _, c := range fetched {
		commentByID[c.ID] = c
	}
	for _, id := range idsToFetch {
		m.loadedCommentIDs[id] = true
	}

	// Match comments to file positions
	loaded := 0
	for _, mapping := range mappings {
		file := m.files[mapping.fileIdx]
		fileLines := getFileLinesForMatching(file)
		if len(fileLines) == 0 {
			continue
		}

		for _, id := range mapping.ids {
			c, ok := commentByID[id]
			if !ok {
				continue
			}

			result := comments.FindCommentPosition(c, fileLines)
			if !result.Found {
				continue
			}

			key := commentKey{
				fileIndex:  mapping.fileIdx,
				newLineNum: result.Line,
			}
			m.appendCommentToThread(key, c)
			loaded++
		}
	}

	return loaded
}

// loadPersistedComments loads the index and matches all comments.
// This is the all-at-once path used in tests and single-commit mode.
func (m *Model) loadPersistedComments() int {
	m.loadCommentIndex()
	return m.matchCommentsForFiles(0, len(m.files))
}

// reloadComments clears cached comment state and re-reads everything from the
// git store. This picks up external changes (e.g. comments resolved via CLI).
func (m *Model) reloadComments() int {
	if m.commentStore == nil {
		return 0
	}
	m.commentIndex = nil
	m.comments = make(map[commentKey][]*comments.Comment)
	m.loadedCommentIDs = make(map[string]bool)
	m.allStoreComments = nil // force re-fetch from store
	m.loadCommentIndex()
	m.loadAllStoreComments()
	// Keep collapsedComments — user's UI toggle state shouldn't reset.
	return m.loadPersistedComments()
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
// Returns the full comment object on success, or nil on error.
func (m *Model) persistComment(key commentKey, text string) *comments.Comment {
	if m.commentStore == nil {
		return nil
	}

	// Get file info
	if key.fileIndex < 0 || key.fileIndex >= len(m.files) {
		return nil
	}
	file := m.files[key.fileIndex]

	// Get context for the line
	fileLines := getFileLinesForMatching(file)
	if key.newLineNum < 1 || key.newLineNum > len(fileLines) {
		return nil
	}

	ctx := comments.ExtractContextForLine(fileLines, key.newLineNum)
	now := time.Now()

	// Check if we're updating an existing persisted comment
	existingID := m.w().commentEditID

	c := &comments.Comment{
		ID:      existingID, // Empty for new, existing ID for update
		Text:    text,
		File:    cleanFilePath(file.NewPath),
		Line:    key.newLineNum,
		Context: ctx,
		Anchor:  ctx.ComputeAnchor(),
		Updated: now,
	}

	// Preserve created time from existing comment, or set for new
	if existingID == "" {
		c.Created = now
	} else if existing := m.findCommentInThread(key, existingID); existing != nil {
		c.Created = existing.Created
	}

	// Get current git state for metadata
	if m.git != nil {
		if branch, err := m.git.CurrentBranch(); err == nil {
			c.Branch = branch
		}
	}

	// Get commit SHA if viewing a specific commit
	if len(m.commits) > 0 && m.commits[0].Info.SHA != "" {
		c.CommitSHA = m.commits[0].Info.SHA
	}

	// Record branch tip when not viewing a specific commit
	if c.CommitSHA == "" && m.git != nil {
		if head, err := m.git.RevParse("HEAD"); err == nil {
			c.BranchHead = head
		}
	}

	// Write to store
	id, err := m.commentStore.WriteComment(c)
	if err != nil {
		return nil
	}
	c.ID = id

	// Keep in-memory index in sync so the comment can be matched
	// against other commits' versions of the same file.
	if m.commentIndex != nil {
		m.commentIndex.Add(c.File, id)
	}

	return c
}

// deletePersistedComment removes a specific comment from the git store by ID.
func (m *Model) deletePersistedComment(key commentKey, commentID string) {
	if m.commentStore == nil || commentID == "" {
		return
	}

	// Delete from store (ignore errors - best effort)
	_ = m.commentStore.DeleteComment(commentID)

	// Keep in-memory index in sync
	if m.commentIndex != nil && key.fileIndex >= 0 && key.fileIndex < len(m.files) {
		file := m.files[key.fileIndex]
		m.commentIndex.Remove(cleanFilePath(file.NewPath), commentID)
	}
}
