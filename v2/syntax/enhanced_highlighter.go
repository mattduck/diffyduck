package syntax

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/mattduck/diffyduck/syntax"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// StyleSpan represents a styling range within a line
type StyleSpan struct {
	Start int         // Start column (0-based)
	End   int         // End column (exclusive)
	Style tcell.Style // tcell style to apply
}

// FileHighlightCache holds parsed syntax tree and computed styles for a file
type FileHighlightCache struct {
	Tree        *tree_sitter.Tree   // Parsed syntax tree
	Language    string              // Language name
	FileContent []string            // Original file lines for reference
	LineStyles  map[int][]StyleSpan // Pre-computed line styles (line number -> spans)
	Timestamp   time.Time           // For cache invalidation
	FilePath    string              // File path for language detection
}

// EnhancedHighlighter provides file-level parsing with lazy line rendering
type EnhancedHighlighter struct {
	baseHighlighter *syntax.Highlighter
	fileCache       map[string]*FileHighlightCache // filePath -> cache
	maxCacheSize    int
	defaultTTL      time.Duration
}

// NewEnhancedHighlighter creates a new enhanced highlighter
func NewEnhancedHighlighter() *EnhancedHighlighter {
	return &EnhancedHighlighter{
		baseHighlighter: syntax.NewHighlighter(),
		fileCache:       make(map[string]*FileHighlightCache),
		maxCacheSize:    10, // Limit to 10 files in cache
		defaultTTL:      5 * time.Minute,
	}
}

// Close cleans up resources
func (eh *EnhancedHighlighter) Close() {
	if eh.baseHighlighter != nil {
		eh.baseHighlighter.Close()
	}
	eh.clearCache()
}

// clearCache removes all cached entries
func (eh *EnhancedHighlighter) clearCache() {
	for _, cache := range eh.fileCache {
		if cache.Tree != nil {
			cache.Tree.Close()
		}
	}
	eh.fileCache = make(map[string]*FileHighlightCache)
}

// cleanExpiredCache removes expired entries
func (eh *EnhancedHighlighter) cleanExpiredCache() {
	now := time.Now()
	for filePath, cache := range eh.fileCache {
		if now.Sub(cache.Timestamp) > eh.defaultTTL {
			if cache.Tree != nil {
				cache.Tree.Close()
			}
			delete(eh.fileCache, filePath)
		}
	}

	// If still over limit, remove oldest entries to make room for new entry
	for len(eh.fileCache) >= eh.maxCacheSize {
		// Find oldest entry
		var oldestPath string
		oldestTime := now

		for filePath, cache := range eh.fileCache {
			if cache.Timestamp.Before(oldestTime) {
				oldestTime = cache.Timestamp
				oldestPath = filePath
			}
		}

		// Remove oldest entry
		if oldestPath != "" {
			if cache, exists := eh.fileCache[oldestPath]; exists {
				if cache.Tree != nil {
					cache.Tree.Close()
				}
				delete(eh.fileCache, oldestPath)
			}
		} else {
			break // Safety break
		}
	}
}

// ParseFile parses an entire file and caches the syntax tree
func (eh *EnhancedHighlighter) ParseFile(filePath string, fileContent []string) error {
	// Clean expired cache entries first
	eh.cleanExpiredCache()

	// Check if already cached and recent
	if cache, exists := eh.fileCache[filePath]; exists {
		if time.Since(cache.Timestamp) < eh.defaultTTL {
			return nil // Already cached and fresh
		}
		// Clean up old cache
		if cache.Tree != nil {
			cache.Tree.Close()
		}
	}

	// Join file content for parsing
	fullContent := strings.Join(fileContent, "\n")

	// Use base highlighter to detect language and get parser
	lang, supported := eh.detectLanguage(filePath)
	if !supported {
		// Store empty cache for unsupported files to avoid repeated attempts
		eh.fileCache[filePath] = &FileHighlightCache{
			Language:    "",
			FileContent: fileContent,
			LineStyles:  make(map[int][]StyleSpan),
			Timestamp:   time.Now(),
			FilePath:    filePath,
		}
		return nil
	}

	parser := eh.getOrCreateParser(lang)
	if parser == nil {
		return fmt.Errorf("failed to create parser for language %s", lang.GetLanguageName())
	}

	// Parse the full file content
	tree := parser.Parse([]byte(fullContent), nil)
	if tree == nil {
		return fmt.Errorf("failed to parse file %s", filePath)
	}

	// Create cache entry
	cache := &FileHighlightCache{
		Tree:        tree,
		Language:    lang.GetLanguageName(),
		FileContent: fileContent,
		LineStyles:  make(map[int][]StyleSpan),
		Timestamp:   time.Now(),
		FilePath:    filePath,
	}

	eh.fileCache[filePath] = cache
	return nil
}

// ParseFilePartial parses only a portion of a file for fast startup
func (eh *EnhancedHighlighter) ParseFilePartial(filePath string, partialContent []string, startLine int) error {
	// Clean expired cache entries first
	eh.cleanExpiredCache()

	// Check if already cached and recent
	if cache, exists := eh.fileCache[filePath]; exists {
		if time.Since(cache.Timestamp) < eh.defaultTTL {
			return nil // Already cached and fresh
		}
		// Clean up old cache
		if cache.Tree != nil {
			cache.Tree.Close()
		}
	}

	// Join partial content for parsing
	fullContent := strings.Join(partialContent, "\n")

	// Use base highlighter to detect language and get parser
	lang, supported := eh.detectLanguage(filePath)
	if !supported {
		// Store empty cache for unsupported files to avoid repeated attempts
		eh.fileCache[filePath] = &FileHighlightCache{
			Language:    "",
			FileContent: partialContent,
			LineStyles:  make(map[int][]StyleSpan),
			Timestamp:   time.Now(),
			FilePath:    filePath,
		}
		return nil
	}

	parser := eh.getOrCreateParser(lang)
	if parser == nil {
		return fmt.Errorf("failed to create parser for language %s", lang.GetLanguageName())
	}

	// Parse the partial file content
	tree := parser.Parse([]byte(fullContent), nil)
	if tree == nil {
		return fmt.Errorf("failed to parse partial file %s", filePath)
	}

	// Create cache entry for partial content
	cache := &FileHighlightCache{
		Tree:        tree,
		Language:    lang.GetLanguageName(),
		FileContent: partialContent,
		LineStyles:  make(map[int][]StyleSpan),
		Timestamp:   time.Now(),
		FilePath:    filePath,
	}

	eh.fileCache[filePath] = cache
	return nil
}

// IsFileParsed checks if a file has been parsed and cached
func (eh *EnhancedHighlighter) IsFileParsed(filePath string) bool {
	cache, exists := eh.fileCache[filePath]
	if !exists {
		return false
	}
	// Check if cache is still valid
	return time.Since(cache.Timestamp) < eh.defaultTTL
}

// GetLineHighlighting returns the highlighted content for a specific line
func (eh *EnhancedHighlighter) GetLineHighlighting(filePath string, lineNumber int, lineContent string) string {
	// Get or create file cache
	cache, exists := eh.fileCache[filePath]
	if !exists {
		// Fallback to single-line highlighting
		return eh.baseHighlighter.HighlightLine(lineContent, filePath)
	}

	// If no tree (unsupported language), return plain content
	if cache.Tree == nil {
		return lineContent
	}

	// Check if we already have styles for this line
	if styles, exists := cache.LineStyles[lineNumber]; exists {
		return eh.applyStylesToLine(lineContent, styles)
	}

	// Compute styles for this line using the cached tree
	styles := eh.computeLineStyles(cache, lineNumber, lineContent)
	cache.LineStyles[lineNumber] = styles

	return eh.applyStylesToLine(lineContent, styles)
}

// GetLineStyles returns just the style spans for a line without applying them
func (eh *EnhancedHighlighter) GetLineStyles(filePath string, lineNumber int, lineContent string) []StyleSpan {
	cache, exists := eh.fileCache[filePath]
	if !exists || cache.Tree == nil {
		return nil // No highlighting available
	}

	if styles, exists := cache.LineStyles[lineNumber]; exists {
		return styles
	}

	// Compute and cache styles
	styles := eh.computeLineStylesOptimized(cache, lineNumber, lineContent)
	cache.LineStyles[lineNumber] = styles
	return styles
}

// computeLineStyles computes syntax highlighting styles for a specific line
func (eh *EnhancedHighlighter) computeLineStyles(cache *FileHighlightCache, lineNumber int, lineContent string) []StyleSpan {
	// Convert line number to byte range in full file content
	var startByte, endByte uint32
	currentLine := 0
	byteOffset := uint32(0)

	for _, line := range cache.FileContent {
		if currentLine == lineNumber-1 { // lineNumber is 1-based
			startByte = byteOffset
			endByte = byteOffset + uint32(len(line))
			break
		}
		byteOffset += uint32(len(line) + 1) // +1 for newline
		currentLine++
	}

	if startByte == endByte {
		return nil // Empty or invalid line
	}

	// Get language for query
	lang, supported := eh.detectLanguage(cache.FilePath)
	if !supported {
		return nil
	}

	query := eh.getOrCreateQuery(lang)
	if query == nil {
		return nil
	}

	// Execute query on the cached tree
	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()

	// Get capture names for this query
	captureNames := query.CaptureNames()

	// Get all captures from the query
	fullContent := strings.Join(cache.FileContent, "\n")
	captures := cursor.Captures(query, cache.Tree.RootNode(), []byte(fullContent))

	var spans []StyleSpan

	// Process captures that intersect with our line
	for match, captureIndex := captures.Next(); match != nil; match, captureIndex = captures.Next() {
		capture := match.Captures[captureIndex]
		captureName := captureNames[capture.Index]
		captureNode := capture.Node

		captureStart := uint32(captureNode.StartByte())
		captureEnd := uint32(captureNode.EndByte())

		// Check if this capture intersects with our line
		if captureEnd > startByte && captureStart < endByte {
			// Calculate column positions within the line
			colStart := 0
			colEnd := len(lineContent)

			if captureStart > startByte {
				colStart = int(captureStart - startByte)
			}
			if captureEnd < endByte {
				colEnd = int(captureEnd - startByte)
			}

			// Ensure bounds are valid
			if colStart < 0 {
				colStart = 0
			}
			if colEnd > len(lineContent) {
				colEnd = len(lineContent)
			}
			if colStart >= colEnd {
				continue
			}

			// Get corresponding style
			style := eh.getCaptureStyle(captureName)

			spans = append(spans, StyleSpan{
				Start: colStart,
				End:   colEnd,
				Style: style,
			})
		}
	}

	return spans
}

// computeLineStylesOptimized efficiently computes styles by running one query and caching all results
func (eh *EnhancedHighlighter) computeLineStylesOptimized(cache *FileHighlightCache, lineNumber int, lineContent string) []StyleSpan {
	// Check if we need to compute all styles at once
	if len(cache.LineStyles) == 0 {
		eh.computeAllLineStyles(cache)
	}

	// Return cached result (may be empty if line has no highlighting)
	if styles, exists := cache.LineStyles[lineNumber]; exists {
		return styles
	}

	// Fallback to single-line computation for lines not covered by bulk computation
	return eh.computeLineStyles(cache, lineNumber, lineContent)
}

// computeAllLineStyles runs one tree-sitter query and computes styles for all lines
func (eh *EnhancedHighlighter) computeAllLineStyles(cache *FileHighlightCache) {
	// Get language for query
	lang, supported := eh.detectLanguage(cache.FilePath)
	if !supported {
		return
	}

	query := eh.getOrCreateQuery(lang)
	if query == nil {
		return
	}

	// Execute query once on the entire tree
	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()

	captureNames := query.CaptureNames()
	fullContent := strings.Join(cache.FileContent, "\n")
	captures := cursor.Captures(query, cache.Tree.RootNode(), []byte(fullContent))

	// Initialize line styles map
	lineStyles := make(map[int][]StyleSpan)

	// Pre-compute byte offsets for each line
	lineOffsets := make([]uint32, len(cache.FileContent)+1)
	byteOffset := uint32(0)
	for i, line := range cache.FileContent {
		lineOffsets[i] = byteOffset
		byteOffset += uint32(len(line) + 1) // +1 for newline
	}
	lineOffsets[len(cache.FileContent)] = byteOffset // End of file

	// Process all captures and distribute to appropriate lines
	for match, captureIndex := captures.Next(); match != nil; match, captureIndex = captures.Next() {
		capture := match.Captures[captureIndex]
		captureName := captureNames[capture.Index]
		captureNode := capture.Node

		captureStart := uint32(captureNode.StartByte())
		captureEnd := uint32(captureNode.EndByte())

		style := eh.getCaptureStyle(captureName)

		// Find which lines this capture affects
		for lineNum := 1; lineNum <= len(cache.FileContent); lineNum++ {
			lineStartByte := lineOffsets[lineNum-1]
			lineEndByte := lineOffsets[lineNum] - 1 // Exclude newline

			// Check if capture intersects with this line
			if captureEnd > lineStartByte && captureStart < lineEndByte {
				// Calculate column positions within the line
				colStart := 0
				colEnd := len(cache.FileContent[lineNum-1])

				if captureStart > lineStartByte {
					colStart = int(captureStart - lineStartByte)
				}
				if captureEnd < lineEndByte {
					colEnd = int(captureEnd - lineStartByte)
				}

				// Ensure bounds are valid
				if colStart < 0 {
					colStart = 0
				}
				lineLen := len(cache.FileContent[lineNum-1])
				if colEnd > lineLen {
					colEnd = lineLen
				}
				if colStart >= colEnd {
					continue
				}

				// Add style span to this line
				if lineStyles[lineNum] == nil {
					lineStyles[lineNum] = make([]StyleSpan, 0, 4) // Pre-allocate small capacity
				}

				lineStyles[lineNum] = append(lineStyles[lineNum], StyleSpan{
					Start: colStart,
					End:   colEnd,
					Style: style,
				})
			}
		}
	}

	// Optimize style spans by merging overlapping ones with same style
	for lineNum := range lineStyles {
		lineStyles[lineNum] = eh.optimizeStyleSpans(lineStyles[lineNum])
	}

	// Store all computed styles
	cache.LineStyles = lineStyles
}

// optimizeStyleSpans merges overlapping spans with the same style
func (eh *EnhancedHighlighter) optimizeStyleSpans(spans []StyleSpan) []StyleSpan {
	if len(spans) <= 1 {
		return spans
	}

	// Sort spans by start position
	for i := 0; i < len(spans)-1; i++ {
		for j := i + 1; j < len(spans); j++ {
			if spans[i].Start > spans[j].Start {
				spans[i], spans[j] = spans[j], spans[i]
			}
		}
	}

	optimized := make([]StyleSpan, 0, len(spans))
	current := spans[0]

	for i := 1; i < len(spans); i++ {
		next := spans[i]

		// Check if we can merge with current span
		if next.Start <= current.End && eh.stylesEqual(current.Style, next.Style) {
			// Merge: extend current span
			if next.End > current.End {
				current.End = next.End
			}
		} else {
			// Can't merge: add current and start new one
			optimized = append(optimized, current)
			current = next
		}
	}

	// Add the final span
	optimized = append(optimized, current)
	return optimized
}

// stylesEqual checks if two tcell styles are equivalent
func (eh *EnhancedHighlighter) stylesEqual(a, b tcell.Style) bool {
	aFg, aBg, aAttrs := a.Decompose()
	bFg, bBg, bAttrs := b.Decompose()
	return aFg == bFg && aBg == bBg && aAttrs == bAttrs
}

// applyStylesToLine applies style spans to a line content
func (eh *EnhancedHighlighter) applyStylesToLine(content string, spans []StyleSpan) string {
	if len(spans) == 0 {
		return content
	}

	// For now, just return the content - we'll apply tcell styles during rendering
	// This is where we'd integrate with tcell's styling system
	return content
}

// getCaptureStyle converts a capture name to a tcell style
func (eh *EnhancedHighlighter) getCaptureStyle(captureName string) tcell.Style {
	// Map common capture names to tcell colors
	switch captureName {
	case "keyword":
		return tcell.StyleDefault.Foreground(tcell.ColorBlue)
	case "string":
		return tcell.StyleDefault.Foreground(tcell.ColorGreen)
	case "comment":
		return tcell.StyleDefault.Foreground(tcell.ColorGray)
	case "number":
		return tcell.StyleDefault.Foreground(tcell.ColorYellow)
	case "function", "function.method", "function.builtin":
		return tcell.StyleDefault.Foreground(tcell.ColorAqua)
	case "type", "type.builtin":
		return tcell.StyleDefault.Foreground(tcell.ColorFuchsia)
	case "constant", "constant.builtin":
		return tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
	case "property":
		return tcell.StyleDefault.Foreground(tcell.ColorAqua)
	case "operator":
		return tcell.StyleDefault.Foreground(tcell.ColorWhite)
	case "punctuation.delimiter":
		return tcell.StyleDefault.Foreground(tcell.ColorWhite)
	default:
		return tcell.StyleDefault
	}
}

// Helper methods that delegate to base highlighter
func (eh *EnhancedHighlighter) detectLanguage(filePath string) (syntax.LanguageDefinition, bool) {
	return eh.baseHighlighter.DetectLanguage(filePath)
}

func (eh *EnhancedHighlighter) getOrCreateParser(lang syntax.LanguageDefinition) *tree_sitter.Parser {
	return eh.baseHighlighter.GetOrCreateParser(lang)
}

func (eh *EnhancedHighlighter) getOrCreateQuery(lang syntax.LanguageDefinition) *tree_sitter.Query {
	return eh.baseHighlighter.GetOrCreateQuery(lang)
}
