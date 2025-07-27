package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/mattduck/diffyduck/aligner"
	"github.com/mattduck/diffyduck/git"
	"github.com/mattduck/diffyduck/syntax"
	"github.com/mattduck/diffyduck/v2/models"
	v2syntax "github.com/mattduck/diffyduck/v2/syntax"
)

// LineCache represents cached rendered content for a line
type LineCache struct {
	Content   string
	Timestamp time.Time
}

// DiffViewport implements a virtual viewport for efficient diff rendering
type DiffViewport struct {
	content             *models.DiffContent
	highlighter         *syntax.Highlighter           // Legacy highlighter for fallback
	enhancedHighlighter *v2syntax.EnhancedHighlighter // New file-level highlighter

	// Viewport state
	offsetY int // First visible line
	offsetX int // Horizontal scroll offset
	width   int // Viewport width
	height  int // Viewport height

	// Caching
	highlightCache map[string]LineCache // Key: fileIndex:lineIndex:isOld
	cacheSize      int                  // Maximum cache entries

	// Performance metrics
	lastRenderTime time.Duration
	renderCount    int
}

const (
	defaultCacheSize  = 1000
	lineNumWidth      = 5
	changeMarkerWidth = 3 // One space with background + two spaces without
	separatorWidth    = 7 // " | " + " | " + "|"
)

// NewDiffViewport creates a new virtual diff viewport
func NewDiffViewport(content *models.DiffContent) *DiffViewport {
	viewport := &DiffViewport{
		content:             content,
		highlighter:         syntax.NewHighlighter(),
		enhancedHighlighter: v2syntax.NewEnhancedHighlighter(),
		highlightCache:      make(map[string]LineCache),
		cacheSize:           defaultCacheSize,
	}

	// Don't pre-parse files - do it lazily when first requested
	// This improves initial load time significantly

	return viewport
}

// preParseFiles parses all files in the diff content for efficient syntax highlighting
func (dv *DiffViewport) preParseFiles() {
	for _, file := range dv.content.Files {
		if file.OldFileType == git.BinaryFile && file.NewFileType == git.BinaryFile {
			continue // Skip binary files
		}

		// Reconstruct old file content with proper line numbers
		if file.OldFileType != git.BinaryFile {
			oldContent := dv.reconstructFileContent(file.AlignedLines, true)
			if len(oldContent) > 0 {
				dv.enhancedHighlighter.ParseFile(file.FileDiff.OldPath, oldContent)
			}
		}

		// Reconstruct new file content with proper line numbers
		if file.NewFileType != git.BinaryFile {
			newContent := dv.reconstructFileContent(file.AlignedLines, false)
			if len(newContent) > 0 {
				dv.enhancedHighlighter.ParseFile(file.FileDiff.NewPath, newContent)
			}
		}
	}
}

// reconstructFileContent rebuilds the full file content from aligned lines
func (dv *DiffViewport) reconstructFileContent(alignedLines []aligner.AlignedLine, isOld bool) []string {
	if len(alignedLines) == 0 {
		return nil
	}

	// Find the maximum line number to determine file size
	maxLineNum := 0
	for _, line := range alignedLines {
		if isOld && line.OldLineNum > maxLineNum {
			maxLineNum = line.OldLineNum
		} else if !isOld && line.NewLineNum > maxLineNum {
			maxLineNum = line.NewLineNum
		}
	}

	if maxLineNum == 0 {
		return nil
	}

	// Create content array with proper size
	content := make([]string, maxLineNum)

	// Fill in the lines at their correct positions
	for _, line := range alignedLines {
		var lineNum int
		var lineContent *string

		if isOld {
			lineNum = line.OldLineNum
			lineContent = line.OldLine
		} else {
			lineNum = line.NewLineNum
			lineContent = line.NewLine
		}

		if lineNum > 0 && lineContent != nil {
			content[lineNum-1] = *lineContent // Convert to 0-based index
		}
	}

	return content
}

// SetSize updates the viewport dimensions
func (dv *DiffViewport) SetSize(width, height int) {
	dv.width = width
	dv.height = height
}

// GetHeight returns the viewport height
func (dv *DiffViewport) GetHeight() int {
	return dv.height
}

// GetOffsets returns the current viewport offsets
func (dv *DiffViewport) GetOffsets() (int, int) {
	return dv.offsetY, dv.offsetX
}

// ScrollVertical scrolls the viewport vertically
func (dv *DiffViewport) ScrollVertical(delta int) {
	newOffset := dv.offsetY + delta
	if newOffset < 0 {
		newOffset = 0
	}
	if newOffset >= dv.content.TotalLines {
		newOffset = dv.content.TotalLines - 1
	}
	dv.offsetY = newOffset
}

// ScrollHorizontal scrolls the viewport horizontally
func (dv *DiffViewport) ScrollHorizontal(delta int) {
	newOffset := dv.offsetX + delta
	if newOffset < 0 {
		newOffset = 0
	}
	dv.offsetX = newOffset
}

// Render draws the visible portion of the diff to the screen
func (dv *DiffViewport) Render(screen tcell.Screen) {
	start := time.Now()
	defer func() {
		dv.lastRenderTime = time.Since(start)
		dv.renderCount++
	}()

	if dv.height <= 0 || dv.width <= 0 {
		return
	}

	// Clear the screen area
	dv.clearScreen(screen)

	// Get visible lines
	visibleLines := dv.content.GetVisibleLines(dv.offsetY, dv.height)

	// Calculate content width per column
	totalSeparators := separatorWidth + 2*(lineNumWidth+changeMarkerWidth)
	contentWidth := (dv.width - totalSeparators) / 2
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Render each visible line
	for row, lineInfo := range visibleLines {
		dv.renderLine(screen, row, lineInfo, contentWidth)
	}
}

// clearScreen clears the viewport area
func (dv *DiffViewport) clearScreen(screen tcell.Screen) {
	for y := 0; y < dv.height; y++ {
		for x := 0; x < dv.width; x++ {
			screen.SetContent(x, y, ' ', nil, tcell.StyleDefault)
		}
	}
}

// renderLine renders a single line to the screen
func (dv *DiffViewport) renderLine(screen tcell.Screen, row int, lineInfo models.LineInfo, contentWidth int) {
	if lineInfo.IsFileHeader() {
		dv.renderFileHeader(screen, row, lineInfo)
		return
	}

	if lineInfo.IsFileSeparator() {
		dv.renderFileSeparator(screen, row)
		return
	}

	// Render content line
	dv.renderContentLine(screen, row, lineInfo, contentWidth)
}

// renderFileHeader renders a file header line
func (dv *DiffViewport) renderFileHeader(screen tcell.Screen, row int, lineInfo models.LineInfo) {
	file := dv.content.Files[lineInfo.FileIndex]
	filename := file.FileDiff.NewPath
	if file.FileDiff.NewPath == "/dev/null" {
		filename = file.FileDiff.OldPath
	}

	// Determine file status
	var marker string
	var style tcell.Style
	if file.FileDiff.OldPath == "/dev/null" {
		marker = "+"
		style = tcell.StyleDefault.Foreground(tcell.ColorGreen)
	} else if file.FileDiff.NewPath == "/dev/null" {
		marker = "-"
		style = tcell.StyleDefault.Foreground(tcell.ColorMaroon)
	} else {
		marker = "~"
		style = tcell.StyleDefault.Foreground(tcell.ColorNavy)
	}

	headerText := fmt.Sprintf("%s %s", marker, filename)
	dv.drawText(screen, 0, row, headerText, style)
}

// renderFileSeparator renders a separator line between files
func (dv *DiffViewport) renderFileSeparator(screen tcell.Screen, row int) {
	separatorText := strings.Repeat("─", dv.width)
	dv.drawText(screen, 0, row, separatorText, tcell.StyleDefault)
}

// renderContentLine renders a diff content line
func (dv *DiffViewport) renderContentLine(screen tcell.Screen, row int, lineInfo models.LineInfo, contentWidth int) {
	line := lineInfo.Line
	col := 0

	// Left side (old content)
	leftContent := ""
	leftContentStyle := tcell.StyleDefault
	leftLineNumStyle := tcell.StyleDefault
	var leftStyleSpans []v2syntax.StyleSpan
	if line.OldLine != nil {
		leftContent = *line.OldLine
		leftStyleSpans = dv.getHighlightedStyleSpans(leftContent, lineInfo.FilePath, true, lineInfo)
		leftStyleSpans = dv.adjustStyleSpansForHorizontalOffset(leftStyleSpans, contentWidth)
		leftContent = dv.applyHorizontalOffset(leftContent, contentWidth)
		// Content has no background highlighting
		leftContentStyle = tcell.StyleDefault
		// Line number style based on change type
		if line.LineType == aligner.Deleted {
			leftLineNumStyle = tcell.StyleDefault.Foreground(tcell.ColorMaroon).Background(tcell.Color16)
		} else if line.LineType == aligner.Modified {
			leftLineNumStyle = tcell.StyleDefault.Foreground(tcell.ColorNavy).Background(tcell.Color16)
		} else {
			leftLineNumStyle = tcell.StyleDefault.Foreground(tcell.ColorGray)
		}
	} else {
		// Empty line number for missing content (only when there's a change on the other side)
		if line.NewLine != nil && (line.LineType == aligner.Added || line.LineType == aligner.Modified) {
			leftLineNumStyle = tcell.StyleDefault.Background(tcell.Color16)
		} else {
			leftLineNumStyle = tcell.StyleDefault.Foreground(tcell.ColorGray)
		}
	}

	// Left line number
	leftLineNum := ""
	if line.OldLine != nil {
		leftLineNum = fmt.Sprintf("%*d", lineNumWidth, line.OldLineNum)
	} else {
		leftLineNum = strings.Repeat(" ", lineNumWidth)
	}
	dv.drawText(screen, col, row, leftLineNum, leftLineNumStyle)
	col += lineNumWidth
	// Add one space with same background as line number
	dv.drawText(screen, col, row, " ", leftLineNumStyle)
	col += 1
	// Add separator spaces with no formatting
	dv.drawText(screen, col, row, "  ", tcell.StyleDefault)
	col += 2

	// Left content with syntax highlighting
	dv.drawTextWithStyleSpans(screen, col, row, leftContent, leftContentStyle, leftStyleSpans)
	col += contentWidth

	// Separator
	dv.drawText(screen, col, row, " | ", tcell.StyleDefault)
	col += 3

	// Right side (new content)
	rightContent := ""
	rightContentStyle := tcell.StyleDefault
	rightLineNumStyle := tcell.StyleDefault
	var rightStyleSpans []v2syntax.StyleSpan
	if line.NewLine != nil {
		rightContent = *line.NewLine
		rightStyleSpans = dv.getHighlightedStyleSpans(rightContent, lineInfo.FilePath, false, lineInfo)
		rightStyleSpans = dv.adjustStyleSpansForHorizontalOffset(rightStyleSpans, contentWidth)
		rightContent = dv.applyHorizontalOffset(rightContent, contentWidth)
		// Content has no background highlighting
		rightContentStyle = tcell.StyleDefault
		// Line number style based on change type
		if line.LineType == aligner.Added {
			rightLineNumStyle = tcell.StyleDefault.Foreground(tcell.ColorGreen).Background(tcell.Color16)
		} else if line.LineType == aligner.Modified {
			rightLineNumStyle = tcell.StyleDefault.Foreground(tcell.ColorNavy).Background(tcell.Color16)
		} else {
			rightLineNumStyle = tcell.StyleDefault.Foreground(tcell.ColorGray)
		}
	} else {
		// Empty line number for missing content (only when there's a change on the other side)
		if line.OldLine != nil && (line.LineType == aligner.Deleted || line.LineType == aligner.Modified) {
			rightLineNumStyle = tcell.StyleDefault.Background(tcell.Color16)
		} else {
			rightLineNumStyle = tcell.StyleDefault.Foreground(tcell.ColorGray)
		}
	}

	// Right line number
	rightLineNum := ""
	if line.NewLine != nil {
		rightLineNum = fmt.Sprintf("%*d", lineNumWidth, line.NewLineNum)
	} else {
		rightLineNum = strings.Repeat(" ", lineNumWidth)
	}
	dv.drawText(screen, col, row, rightLineNum, rightLineNumStyle)
	col += lineNumWidth
	// Add one space with same background as line number
	dv.drawText(screen, col, row, " ", rightLineNumStyle)
	col += 1
	// Add separator spaces with no formatting
	dv.drawText(screen, col, row, "  ", tcell.StyleDefault)
	col += 2

	// Right content with syntax highlighting
	dv.drawTextWithStyleSpans(screen, col, row, rightContent, rightContentStyle, rightStyleSpans)
}

// getHighlightedStyleSpans returns style spans for a line, using cache when possible
func (dv *DiffViewport) getHighlightedStyleSpans(content, filePath string, isOld bool, lineInfo models.LineInfo) []v2syntax.StyleSpan {
	// Use enhanced highlighter to get style spans
	if dv.enhancedHighlighter != nil {
		// Ensure file is parsed (lazy parsing)
		dv.ensureFileParsed(lineInfo.FileIndex)

		// Calculate line number within the file (1-based)
		lineNumber := lineInfo.LineIndex + 1
		if isOld && lineInfo.Line.OldLineNum > 0 {
			lineNumber = lineInfo.Line.OldLineNum
		} else if !isOld && lineInfo.Line.NewLineNum > 0 {
			lineNumber = lineInfo.Line.NewLineNum
		}

		return dv.enhancedHighlighter.GetLineStyles(filePath, lineNumber, content)
	}

	// No enhanced highlighter available
	return nil
}

// ensureFileParsed lazily parses a file if it hasn't been parsed yet
func (dv *DiffViewport) ensureFileParsed(fileIndex int) {
	if fileIndex >= len(dv.content.Files) {
		return
	}

	file := dv.content.Files[fileIndex]
	if file.OldFileType == git.BinaryFile && file.NewFileType == git.BinaryFile {
		return // Skip binary files
	}

	// Check if old file is already parsed
	if file.OldFileType != git.BinaryFile {
		oldContent := dv.reconstructFileContent(file.AlignedLines, true)
		if len(oldContent) > 0 {
			dv.enhancedHighlighter.ParseFile(file.FileDiff.OldPath, oldContent)
		}
	}

	// Check if new file is already parsed
	if file.NewFileType != git.BinaryFile {
		newContent := dv.reconstructFileContent(file.AlignedLines, false)
		if len(newContent) > 0 {
			dv.enhancedHighlighter.ParseFile(file.FileDiff.NewPath, newContent)
		}
	}
}

// getHighlightedContent returns highlighted content for a line (legacy method for caching)
func (dv *DiffViewport) getHighlightedContent(content, filePath string, isOld bool, lineInfo models.LineInfo) string {
	// Create cache key
	cacheKey := fmt.Sprintf("%d:%d:%t", lineInfo.FileIndex, lineInfo.LineIndex, isOld)

	// Check cache first
	if cached, exists := dv.highlightCache[cacheKey]; exists {
		// Use cached version if it's recent (within 1 minute)
		if time.Since(cached.Timestamp) < time.Minute {
			return cached.Content
		}
	}

	// For now, we'll use the style spans approach instead of returning styled text
	// This method is kept for backward compatibility
	highlighted := content

	// Cache the result
	dv.cacheHighlightedContent(cacheKey, highlighted)

	return highlighted
}

// cacheHighlightedContent stores highlighted content in cache
func (dv *DiffViewport) cacheHighlightedContent(key, content string) {
	// Clean cache if it's getting too large
	if len(dv.highlightCache) >= dv.cacheSize {
		dv.cleanCache()
	}

	dv.highlightCache[key] = LineCache{
		Content:   content,
		Timestamp: time.Now(),
	}
}

// cleanCache removes old entries from the highlight cache
func (dv *DiffViewport) cleanCache() {
	// Remove entries older than 2 minutes
	cutoff := time.Now().Add(-2 * time.Minute)
	for key, cached := range dv.highlightCache {
		if cached.Timestamp.Before(cutoff) {
			delete(dv.highlightCache, key)
		}
	}

	// If still too large, remove half randomly
	if len(dv.highlightCache) >= dv.cacheSize {
		count := 0
		target := dv.cacheSize / 2
		for key := range dv.highlightCache {
			if count >= target {
				break
			}
			delete(dv.highlightCache, key)
			count++
		}
	}
}

// applyHorizontalOffset applies horizontal scrolling to content
func (dv *DiffViewport) applyHorizontalOffset(content string, width int) string {
	if dv.offsetX == 0 {
		// No horizontal scroll, just truncate to width
		if len(content) > width {
			return content[:width]
		}
		return content + strings.Repeat(" ", width-len(content))
	}

	// Apply horizontal offset
	if len(content) <= dv.offsetX {
		return strings.Repeat(" ", width)
	}

	result := content[dv.offsetX:]
	if len(result) > width {
		result = result[:width]
	} else if len(result) < width {
		result = result + strings.Repeat(" ", width-len(result))
	}

	return result
}

// adjustStyleSpansForHorizontalOffset adjusts style spans to account for horizontal scrolling
func (dv *DiffViewport) adjustStyleSpansForHorizontalOffset(spans []v2syntax.StyleSpan, width int) []v2syntax.StyleSpan {
	if len(spans) == 0 || dv.offsetX == 0 {
		return spans
	}

	var adjustedSpans []v2syntax.StyleSpan

	for _, span := range spans {
		// Adjust span positions for horizontal offset
		newStart := span.Start - dv.offsetX
		newEnd := span.End - dv.offsetX

		// Skip spans that are completely off-screen to the left
		if newEnd <= 0 {
			continue
		}

		// Skip spans that are completely off-screen to the right
		if newStart >= width {
			continue
		}

		// Clamp spans to visible area
		if newStart < 0 {
			newStart = 0
		}
		if newEnd > width {
			newEnd = width
		}

		// Only add spans that have positive width
		if newStart < newEnd {
			adjustedSpans = append(adjustedSpans, v2syntax.StyleSpan{
				Start: newStart,
				End:   newEnd,
				Style: span.Style,
			})
		}
	}

	return adjustedSpans
}

// drawText draws text to the screen at the specified position
func (dv *DiffViewport) drawText(screen tcell.Screen, x, y int, text string, style tcell.Style) {
	for i, r := range text {
		if x+i >= dv.width {
			break
		}
		screen.SetContent(x+i, y, r, nil, style)
	}
}

// drawTextWithStyleSpans draws text with syntax highlighting style spans
func (dv *DiffViewport) drawTextWithStyleSpans(screen tcell.Screen, x, y int, text string, baseStyle tcell.Style, styleSpans []v2syntax.StyleSpan) {
	if len(styleSpans) == 0 {
		// No highlighting, use base style
		dv.drawText(screen, x, y, text, baseStyle)
		return
	}

	// Convert text to runes for proper indexing
	runes := []rune(text)

	// Draw each character with appropriate style
	for i, r := range runes {
		if x+i >= dv.width {
			break
		}

		// Find the appropriate style for this character position
		charStyle := baseStyle
		for _, span := range styleSpans {
			if i >= span.Start && i < span.End {
				// Merge syntax highlighting with base style
				fg, bg, attrs := span.Style.Decompose()
				if fg == tcell.ColorDefault {
					fg, _, _ = baseStyle.Decompose()
				}
				if bg == tcell.ColorDefault {
					_, bg, _ = baseStyle.Decompose()
				}
				charStyle = tcell.StyleDefault.Foreground(fg).Background(bg).Attributes(attrs)
				break
			}
		}

		screen.SetContent(x+i, y, r, nil, charStyle)
	}
}

// GetRenderStats returns performance statistics
func (dv *DiffViewport) GetRenderStats() (time.Duration, int) {
	return dv.lastRenderTime, dv.renderCount
}

// Close cleans up resources
func (dv *DiffViewport) Close() {
	if dv.highlighter != nil {
		dv.highlighter.Close()
	}
	if dv.enhancedHighlighter != nil {
		dv.enhancedHighlighter.Close()
	}
}
