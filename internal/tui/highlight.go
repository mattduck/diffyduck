package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/diffyduck/pkg/highlight"
	"github.com/user/diffyduck/pkg/sidebyside"
	"github.com/user/diffyduck/pkg/structure"
)

// RequestHighlight returns a command that parses syntax highlighting for a file.
// This runs asynchronously to avoid blocking the UI.
// Also extracts code structure for breadcrumbs.
func (m Model) RequestHighlight(fileIndex int) tea.Cmd {
	gen := m.reloadGen
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

		if m.highlighter == nil {
			return nil
		}

		var oldSpans, newSpans []highlight.Span
		var oldStructure, newStructure *structure.Map

		// Parse old content if available (with structure for structural diff)
		if len(fp.OldContent) > 0 {
			content := []byte(strings.Join(fp.OldContent, "\n"))
			spans, structMap, _ := m.highlighter.HighlightWithStructure(filename, content)
			oldSpans = spans
			oldStructure = structMap
		}

		// Parse new content if available (with structure for breadcrumbs)
		if len(fp.NewContent) > 0 {
			content := []byte(strings.Join(fp.NewContent, "\n"))
			spans, structMap, _ := m.highlighter.HighlightWithStructure(filename, content)
			newSpans = spans
			newStructure = structMap
		}

		// Convert to message format
		msg := HighlightReadyMsg{
			FileIndex:    fileIndex,
			OldSpans:     convertSpans(oldSpans),
			NewSpans:     convertSpans(newSpans),
			OldStructure: convertStructure(oldStructure),
			NewStructure: convertStructure(newStructure),
			Gen:          gen,
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

// highlightFileSync synchronously highlights a file from its full content.
// Used when expanding files/commits to ensure highlighting is ready before render.
func (m *Model) highlightFileSync(fileIndex int) {
	if fileIndex < 0 || fileIndex >= len(m.files) {
		return
	}

	fp := m.files[fileIndex]

	// Determine filename for language detection
	filename := fp.NewPath
	if filename == "/dev/null" {
		filename = fp.OldPath
	}

	if m.highlighter == nil {
		return
	}

	var oldSpans, newSpans []highlight.Span
	var oldStructure, newStructure *structure.Map

	if len(fp.OldContent) > 0 {
		content := []byte(strings.Join(fp.OldContent, "\n"))
		spans, structMap, _ := m.highlighter.HighlightWithStructure(filename, content)
		oldSpans = spans
		oldStructure = structMap
	}

	if len(fp.NewContent) > 0 {
		content := []byte(strings.Join(fp.NewContent, "\n"))
		spans, structMap, _ := m.highlighter.HighlightWithStructure(filename, content)
		newSpans = spans
		newStructure = structMap
	}

	// Store directly via storeHighlightSpans (reusing the same logic as the async path)
	msg := HighlightReadyMsg{
		FileIndex:    fileIndex,
		OldSpans:     convertSpans(oldSpans),
		NewSpans:     convertSpans(newSpans),
		OldStructure: convertStructure(oldStructure),
		NewStructure: convertStructure(newStructure),
	}
	m.storeHighlightSpans(msg)
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

// convertStructure converts structure.Map to []StructureEntry (avoiding import in messages.go).
func convertStructure(m *structure.Map) []StructureEntry {
	if m == nil || len(m.Entries) == 0 {
		return nil
	}
	result := make([]StructureEntry, len(m.Entries))
	for i, e := range m.Entries {
		result[i] = StructureEntry{
			StartLine:  e.StartLine,
			EndLine:    e.EndLine,
			Name:       e.Name,
			Kind:       e.Kind,
			Receiver:   e.Receiver,
			Params:     e.Params,
			ReturnType: e.ReturnType,
		}
	}
	return result
}

// storeHighlightSpans stores highlight spans, structural diff, and expands semantic context.
// Returns true if the row layout changed (semantic context expansion or structural diff added rows).
func (m *Model) storeHighlightSpans(msg HighlightReadyMsg) bool {
	m.highlightSpans[msg.FileIndex] = &FileHighlight{
		OldSpans: unconvertSpans(msg.OldSpans),
		NewSpans: unconvertSpans(msg.NewSpans),
	}

	oldStruct := unconvertStructure(msg.OldStructure)
	newStruct := unconvertStructure(msg.NewStructure)

	// Compute structural diff if we have structure for at least one side
	var structDiff *structure.StructuralDiff
	if oldStruct != nil || newStruct != nil {
		// Extract changed lines from the file's Pairs
		addedLines, removedLines := m.extractChangedLines(msg.FileIndex)
		structDiff = structure.ComputeDiff(oldStruct, newStruct, addedLines, removedLines)
	}

	// Store structure (even if empty) to mark file as processed.
	// This prevents filesNeedingStructure from re-processing already-loaded files.
	// OldStructure is used for structural diff, NewStructure for breadcrumbs.
	m.structureMaps[msg.FileIndex] = &FileStructure{
		OldStructure:   oldStruct,
		NewStructure:   newStruct,
		StructuralDiff: structDiff,
	}

	// Expand context to include nearby scope boundaries (function/class starts)
	pairsChanged := false
	if msg.FileIndex >= 0 && msg.FileIndex < len(m.files) {
		pairsBefore := len(m.files[msg.FileIndex].Pairs)
		expandSemanticContext(&m.files[msg.FileIndex], newStruct, SemanticContextThreshold)
		pairsChanged = len(m.files[msg.FileIndex].Pairs) != pairsBefore
		// Snapshot pairs after semantic expansion so fold toggle can restore them
		m.files[msg.FileIndex].SaveOriginalPairs()
		// Recompute move detection for this commit if active — semantic context
		// insertion shifts pair indices, so cached results would map to wrong lines.
		m.recomputeMoveDetectIfActive(msg.FileIndex)
	}

	// Check if structural diff would add visible rows.
	// Structural diff rows appear under file headers, which are only visible
	// when the commit is not folded. Skip for folded commits to avoid
	// unnecessary cache rebuilds in log mode.
	layoutChanged := pairsChanged
	if structDiff != nil && structDiff.HasChanges() {
		commitIdx := m.commitForFile(msg.FileIndex)
		if commitIdx >= 0 && commitIdx < len(m.commits) {
			if m.commitFoldLevel(commitIdx) != sidebyside.CommitFolded {
				layoutChanged = true
			}
		} else {
			// No commit structure (e.g., diff mode) - always mark changed
			layoutChanged = true
		}
	}
	if layoutChanged {
		// Recalculate totalLines so scroll limits are correct.
		// Just invalidating rowsCacheValid is not enough because totalLines
		// is used directly by maxScroll() without triggering a cache rebuild.
		m.calculateTotalLines()
	}
	return layoutChanged
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

// unconvertStructure converts []StructureEntry back to *structure.Map.
func unconvertStructure(entries []StructureEntry) *structure.Map {
	if len(entries) == 0 {
		return nil
	}
	structEntries := make([]structure.Entry, len(entries))
	for i, e := range entries {
		structEntries[i] = structure.Entry{
			StartLine:  e.StartLine,
			EndLine:    e.EndLine,
			Name:       e.Name,
			Kind:       e.Kind,
			Receiver:   e.Receiver,
			Params:     e.Params,
			ReturnType: e.ReturnType,
		}
	}
	return structure.NewMap(structEntries)
}

// getLineSpans returns syntax highlight spans for a specific line in a file.
// lineNum is 1-based line number, isOld indicates old vs new content side.
// The returned spans have byte offsets relative to the line start.
// Returns nil if full file content hasn't been loaded and highlighted yet.
func (m Model) getLineSpans(fileIndex int, lineNum int, isOld bool) []highlight.Span {
	if lineNum <= 0 {
		return nil
	}
	return m.getLineSpansFromFullContent(fileIndex, lineNum, isOld)
}

// getLineSpansFromFullContent returns spans from full file content highlighting.
func (m Model) getLineSpansFromFullContent(fileIndex int, lineNum int, isOld bool) []highlight.Span {
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

// extractChangedLines extracts the added and removed line numbers from a file's Pairs.
// Returns maps of 1-based line numbers that were added (in new) and removed (from old).
func (m Model) extractChangedLines(fileIndex int) (addedLines, removedLines map[int]bool) {
	if fileIndex < 0 || fileIndex >= len(m.files) {
		return nil, nil
	}

	fp := m.files[fileIndex]
	addedLines = make(map[int]bool)
	removedLines = make(map[int]bool)

	for _, pair := range fp.Pairs {
		// Check old side for removed lines
		if pair.Old.Type == sidebyside.Removed && pair.Old.Num > 0 {
			removedLines[pair.Old.Num] = true
		}
		// Check new side for added lines
		if pair.New.Type == sidebyside.Added && pair.New.Num > 0 {
			addedLines[pair.New.Num] = true
		}
	}

	return addedLines, removedLines
}
