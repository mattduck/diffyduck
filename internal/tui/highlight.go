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

// RequestHighlightFromPairs returns a command that parses syntax highlighting from Pairs content.
// This is used for normal (non-expanded) view where full file content isn't available.
// Also extracts code structure for breadcrumbs.
func (m Model) RequestHighlightFromPairs(fileIndex int) tea.Cmd {
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

		// Build concatenated content from Pairs for both sides
		oldContent, oldLineStarts, oldLineLens := buildContentFromPairs(fp.Pairs, true)
		newContent, newLineStarts, newLineLens := buildContentFromPairs(fp.Pairs, false)

		var oldSpans, newSpans []HighlightSpan

		// Parse old content if we have any
		// Note: We use Highlight() not HighlightWithStructure() because pairs-based
		// structure has wrong line numbers (relative to concatenated content, not source).
		// Structure extraction requires full file content for accurate line mapping.
		if len(oldContent) > 0 {
			spans, _ := m.highlighter.Highlight(filename, oldContent)
			oldSpans = convertSpans(spans)
		}

		// Parse new content if we have any
		if len(newContent) > 0 {
			spans, _ := m.highlighter.Highlight(filename, newContent)
			newSpans = convertSpans(spans)
		}

		return PairsHighlightReadyMsg{
			FileIndex:     fileIndex,
			OldSpans:      oldSpans,
			NewSpans:      newSpans,
			OldLineStarts: oldLineStarts,
			NewLineStarts: newLineStarts,
			OldLineLens:   oldLineLens,
			NewLineLens:   newLineLens,
		}
	}
}

// RequestHighlightFromPairsAll returns a command that parses all files from Pairs.
func (m Model) RequestHighlightFromPairsAll() tea.Cmd {
	var cmds []tea.Cmd
	for i := range m.files {
		cmds = append(cmds, m.RequestHighlightFromPairs(i))
	}
	return tea.Batch(cmds...)
}

// RequestHighlightFromPairsExcept returns a command that parses all files except the given indices.
func (m Model) RequestHighlightFromPairsExcept(skip map[int]bool) tea.Cmd {
	var cmds []tea.Cmd
	for i := range m.files {
		if !skip[i] {
			cmds = append(cmds, m.RequestHighlightFromPairs(i))
		}
	}
	return tea.Batch(cmds...)
}

// highlightPairsSync synchronously highlights a file's Pairs content.
// This is used for the initial file to ensure first render has highlighting.
// Also extracts code structure for breadcrumbs.
func (m *Model) highlightPairsSync(fileIndex int) {
	if fileIndex < 0 || fileIndex >= len(m.files) {
		return
	}

	fp := m.files[fileIndex]

	// Determine filename for language detection
	filename := fp.NewPath
	if filename == "/dev/null" {
		filename = fp.OldPath
	}

	// Check if highlighter supports this file type
	if m.highlighter == nil || !m.highlighter.SupportsFile(filename) {
		return
	}

	// Build concatenated content from Pairs for both sides
	oldContent, oldLineStarts, oldLineLens := buildContentFromPairs(fp.Pairs, true)
	newContent, newLineStarts, newLineLens := buildContentFromPairs(fp.Pairs, false)

	var oldSpans, newSpans []highlight.Span

	// Parse old content if we have any
	// Note: We use Highlight() not HighlightWithStructure() because pairs-based
	// structure has wrong line numbers (relative to concatenated content, not source).
	// Structure extraction requires full file content for accurate line mapping.
	if len(oldContent) > 0 {
		oldSpans, _ = m.highlighter.Highlight(filename, oldContent)
	}

	// Parse new content if we have any
	if len(newContent) > 0 {
		newSpans, _ = m.highlighter.Highlight(filename, newContent)
	}

	// Store directly (no structure for pairs-based - requires full content)
	m.pairsHighlightSpans[fileIndex] = &PairsFileHighlight{
		OldSpans:      oldSpans,
		NewSpans:      newSpans,
		OldLineStarts: oldLineStarts,
		NewLineStarts: newLineStarts,
		OldLineLens:   oldLineLens,
		NewLineLens:   newLineLens,
	}
}

// buildContentFromPairs extracts line content from Pairs and builds a concatenated string.
// Returns the content bytes, a map of line number to byte offset, and line number to length.
// If isOld is true, extracts from Left side; otherwise from Right side.
func buildContentFromPairs(pairs []sidebyside.LinePair, isOld bool) ([]byte, map[int]int, map[int]int) {
	lineStarts := make(map[int]int)
	lineLens := make(map[int]int)

	var buf []byte
	for _, pair := range pairs {
		var line sidebyside.Line
		if isOld {
			line = pair.Old
		} else {
			line = pair.New
		}

		// Skip empty lines (no line number)
		if line.Num == 0 {
			continue
		}

		// Record where this line starts in the concatenated content
		lineStarts[line.Num] = len(buf)
		lineLens[line.Num] = len(line.Content)

		buf = append(buf, line.Content...)
		buf = append(buf, '\n')
	}

	return buf, lineStarts, lineLens
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

// storeHighlightSpans stores the spans and structure from a HighlightReadyMsg into the model.
func (m *Model) storeHighlightSpans(msg HighlightReadyMsg) {
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

	// Invalidate row cache if structural diff would be visible.
	// Structural diff rows appear under file headers, which are only visible
	// when the commit is not folded. Skip invalidation for folded commits
	// to avoid unnecessary cache rebuilds in log mode.
	if structDiff != nil && structDiff.HasChanges() {
		commitIdx := m.commitForFile(msg.FileIndex)
		if commitIdx >= 0 && commitIdx < len(m.commits) {
			if m.commits[commitIdx].FoldLevel != sidebyside.CommitFolded {
				m.rowsCacheValid = false
			}
		} else {
			// No commit structure (e.g., diff mode) - always invalidate
			m.rowsCacheValid = false
		}
	}
}

// storePairsHighlightSpans stores the spans and structure from a PairsHighlightReadyMsg into the model.
func (m *Model) storePairsHighlightSpans(msg PairsHighlightReadyMsg) {
	m.pairsHighlightSpans[msg.FileIndex] = &PairsFileHighlight{
		OldSpans:      unconvertSpans(msg.OldSpans),
		NewSpans:      unconvertSpans(msg.NewSpans),
		OldLineStarts: msg.OldLineStarts,
		NewLineStarts: msg.NewLineStarts,
		OldLineLens:   msg.OldLineLens,
		NewLineLens:   msg.NewLineLens,
	}
	// Note: Pairs-based structure is not stored because line numbers would be
	// relative to concatenated content, not source. Structure requires full content.
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
// It first tries full-content spans, then falls back to pairs-based spans.
func (m Model) getLineSpans(fileIndex int, lineNum int, isOld bool) []highlight.Span {
	if lineNum <= 0 {
		return nil
	}

	// Try full-content spans first (available when file is expanded)
	if spans := m.getLineSpansFromFullContent(fileIndex, lineNum, isOld); spans != nil {
		return spans
	}

	// Fall back to pairs-based spans (available for normal view)
	return m.getLineSpansFromPairs(fileIndex, lineNum, isOld)
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

// getLineSpansFromPairs returns spans from pairs-based highlighting.
func (m Model) getLineSpansFromPairs(fileIndex int, lineNum int, isOld bool) []highlight.Span {
	pfh, ok := m.pairsHighlightSpans[fileIndex]
	if !ok || pfh == nil {
		return nil
	}

	var allSpans []highlight.Span
	var lineStarts, lineLens map[int]int

	if isOld {
		allSpans = pfh.OldSpans
		lineStarts = pfh.OldLineStarts
		lineLens = pfh.OldLineLens
	} else {
		allSpans = pfh.NewSpans
		lineStarts = pfh.NewLineStarts
		lineLens = pfh.NewLineLens
	}

	if len(allSpans) == 0 {
		return nil
	}

	// Look up this line's position in the concatenated content
	lineStart, ok := lineStarts[lineNum]
	if !ok {
		return nil // This line isn't in the pairs
	}

	lineLen, ok := lineLens[lineNum]
	if !ok {
		return nil
	}

	lineEnd := lineStart + lineLen

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
