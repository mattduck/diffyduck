package syntax

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattduck/diffyduck/syntax/languages"
	"github.com/mattduck/diffyduck/types"
	"github.com/sergi/go-diff/diffmatchpatch"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type Highlighter struct {
	parsers       map[string]*tree_sitter.Parser
	languages     map[string]LanguageDefinition
	queries       map[string]*tree_sitter.Query
	extensionMap  map[string]string         // file extension -> language name
	captureStyles map[string]lipgloss.Style // capture name -> style
}

func NewHighlighter() *Highlighter {
	h := &Highlighter{
		parsers:       make(map[string]*tree_sitter.Parser),
		languages:     make(map[string]LanguageDefinition),
		queries:       make(map[string]*tree_sitter.Query),
		extensionMap:  make(map[string]string),
		captureStyles: make(map[string]lipgloss.Style),
	}

	// Initialize capture styles
	h.initializeCaptureStyles()

	// Register Go language by default
	h.RegisterLanguage(languages.NewGoLanguage())

	// Register Python language
	h.RegisterLanguage(languages.NewPythonLanguage())

	return h
}

func (h *Highlighter) Close() {
	for _, parser := range h.parsers {
		if parser != nil {
			parser.Close()
		}
	}
	h.parsers = make(map[string]*tree_sitter.Parser)

	for _, query := range h.queries {
		if query != nil {
			query.Close()
		}
	}
	h.queries = make(map[string]*tree_sitter.Query)
}

// initializeCaptureStyles sets up the mapping from capture names to styles
func (h *Highlighter) initializeCaptureStyles() {
	h.captureStyles["keyword"] = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))                     // blue
	h.captureStyles["string"] = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))                      // gray
	h.captureStyles["comment"] = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))                     // gray
	h.captureStyles["constant.builtin"] = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true) // bold yellow
	h.captureStyles["number"] = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)           // bold yellow
	h.captureStyles["function"] = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))                   // bright blue
	h.captureStyles["function.method"] = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))            // bright blue
	h.captureStyles["function.builtin"] = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))           // bright blue
	h.captureStyles["type"] = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))                       // bright magenta
	h.captureStyles["type.builtin"] = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))               // bright magenta
	h.captureStyles["property"] = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))                   // bright cyan
	h.captureStyles["variable"] = lipgloss.NewStyle()                                                    // default color
	h.captureStyles["constant"] = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)         // bold yellow
	h.captureStyles["operator"] = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))                    // white
	h.captureStyles["punctuation.delimiter"] = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))       // white
}

// RegisterLanguage adds a new language to the highlighter
func (h *Highlighter) RegisterLanguage(lang LanguageDefinition) {
	langName := lang.GetLanguageName()
	h.languages[langName] = lang

	// Map file extensions to language name
	for _, ext := range lang.GetFileExtensions() {
		h.extensionMap[strings.ToLower(ext)] = langName
	}
}

// detectLanguage determines the language from a file path
func (h *Highlighter) detectLanguage(filePath string) (LanguageDefinition, bool) {
	ext := strings.ToLower(filepath.Ext(filePath))
	langName, exists := h.extensionMap[ext]
	if !exists {
		return nil, false
	}

	lang, exists := h.languages[langName]
	return lang, exists
}

// getOrCreateParser gets an existing parser or creates a new one for the language
func (h *Highlighter) getOrCreateParser(lang LanguageDefinition) *tree_sitter.Parser {
	langName := lang.GetLanguageName()

	if parser, exists := h.parsers[langName]; exists {
		return parser
	}

	// Create new parser for this language
	parser := tree_sitter.NewParser()
	parser.SetLanguage(tree_sitter.NewLanguage(lang.GetLanguage()))
	h.parsers[langName] = parser

	return parser
}

// getOrCreateQuery gets an existing query or creates a new one for the language
func (h *Highlighter) getOrCreateQuery(lang LanguageDefinition) *tree_sitter.Query {
	langName := lang.GetLanguageName()

	if query, exists := h.queries[langName]; exists {
		return query
	}

	// Create new query for this language
	queryString := lang.GetHighlightQuery()
	query, err := tree_sitter.NewQuery(tree_sitter.NewLanguage(lang.GetLanguage()), queryString)
	if err != nil {
		// If query creation fails, return nil - highlighting will fall back to plain text
		return nil
	}

	h.queries[langName] = query
	return query
}

func (h *Highlighter) HighlightLine(content, filePath string) string {
	if content == "" {
		return content
	}

	// Detect language from file path
	lang, supported := h.detectLanguage(filePath)
	if !supported {
		return content // no highlighting for unsupported languages
	}

	// Get or create parser for this language
	parser := h.getOrCreateParser(lang)

	// Parse the line content
	tree := parser.Parse([]byte(content), nil)
	if tree == nil {
		return content // fallback on parse error
	}
	defer tree.Close()

	root := tree.RootNode()
	return h.highlightWithQuery(content, root, lang)
}

func (h *Highlighter) highlightWithQuery(content string, node *tree_sitter.Node, lang LanguageDefinition) string {
	// Get or create query for this language
	query := h.getOrCreateQuery(lang)
	if query == nil {
		return content // fallback if query creation failed
	}

	contentBytes := []byte(content)
	result := make([]byte, 0, len(contentBytes)*2) // pre-allocate with extra space for ANSI codes
	lastEnd := uint32(0)

	// Execute query and collect captures
	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()

	// Get capture names for this query
	captureNames := query.CaptureNames()

	// Get all captures from the query
	captures := cursor.Captures(query, node, contentBytes)

	// Process captures in order
	for match, captureIndex := captures.Next(); match != nil; match, captureIndex = captures.Next() {
		capture := match.Captures[captureIndex]
		captureName := captureNames[capture.Index]
		captureNode := capture.Node

		start := uint32(captureNode.StartByte())
		end := uint32(captureNode.EndByte())

		// Skip if this capture overlaps with already processed content
		if start < lastEnd {
			continue
		}

		// Add content before this capture
		if start > lastEnd {
			result = append(result, contentBytes[lastEnd:start]...)
		}

		// Apply styling for this capture
		if style, exists := h.captureStyles[captureName]; exists {
			nodeText := string(contentBytes[start:end])
			styledText := style.Render(nodeText)
			result = append(result, []byte(styledText)...)
		} else {
			// No style defined, add text as-is
			result = append(result, contentBytes[start:end]...)
		}

		lastEnd = end
	}

	// Append any remaining content
	if lastEnd < uint32(len(contentBytes)) {
		result = append(result, contentBytes[lastEnd:]...)
	}

	return string(result)
}

func (h *Highlighter) HighlightLineWithWordDiff(content, filePath string, segments []types.DiffSegment) string {
	if content == "" || len(segments) == 0 {
		return h.HighlightLine(content, filePath)
	}

	// Detect language from file path
	lang, supported := h.detectLanguage(filePath)
	if !supported {
		// No syntax highlighting, just apply word diff styling
		return h.applyWordDiffStyling(segments)
	}

	// Get or create parser for this language
	parser := h.getOrCreateParser(lang)

	// Parse the line content
	tree := parser.Parse([]byte(content), nil)
	if tree == nil {
		// Parse error, fallback to word diff only
		return h.applyWordDiffStyling(segments)
	}
	defer tree.Close()

	root := tree.RootNode()
	return h.highlightWithQueryAndWordDiff(content, root, lang, segments)
}

func (h *Highlighter) applyWordDiffStyling(segments []types.DiffSegment) string {
	var result strings.Builder
	backgroundStyle := lipgloss.NewStyle().Background(lipgloss.Color("16"))

	for _, segment := range segments {
		if segment.Type == diffmatchpatch.DiffDelete || segment.Type == diffmatchpatch.DiffInsert {
			result.WriteString(backgroundStyle.Render(segment.Text))
		} else {
			result.WriteString(segment.Text)
		}
	}

	return result.String()
}

func (h *Highlighter) highlightWithQueryAndWordDiff(content string, node *tree_sitter.Node, lang LanguageDefinition, segments []types.DiffSegment) string {
	// Build a map of text positions to diff types for quick lookup
	diffMap := make(map[int]diffmatchpatch.Operation)
	pos := 0
	for _, segment := range segments {
		segmentEnd := pos + len(segment.Text)
		for i := pos; i < segmentEnd; i++ {
			diffMap[i] = segment.Type
		}
		pos = segmentEnd
	}

	// Get or create query for this language
	query := h.getOrCreateQuery(lang)
	if query == nil {
		// fallback to word diff only if query creation failed
		return h.applyWordDiffStyling(segments)
	}

	contentBytes := []byte(content)
	result := make([]byte, 0, len(contentBytes)*3) // pre-allocate with extra space for ANSI codes
	lastEnd := uint32(0)

	// Execute query and collect captures
	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()

	// Get capture names for this query
	captureNames := query.CaptureNames()

	// Get all captures from the query
	captures := cursor.Captures(query, node, contentBytes)

	// Process captures in order
	for match, captureIndex := captures.Next(); match != nil; match, captureIndex = captures.Next() {
		capture := match.Captures[captureIndex]
		captureName := captureNames[capture.Index]
		captureNode := capture.Node

		start := uint32(captureNode.StartByte())
		end := uint32(captureNode.EndByte())

		// Skip if this capture overlaps with already processed content
		if start < lastEnd {
			continue
		}

		// Add content before this capture
		if start > lastEnd {
			beforeContent := contentBytes[lastEnd:start]
			result = append(result, h.applyDiffStylingToBytes(beforeContent, int(lastEnd), diffMap)...)
		}

		// Apply combined styling for this capture (syntax + word diff)
		if style, exists := h.captureStyles[captureName]; exists {
			nodeText := string(contentBytes[start:end])
			styledText := h.applyCombinedStyling(nodeText, int(start), &style, diffMap)
			result = append(result, []byte(styledText)...)
		} else {
			// No style defined, apply word diff styling only
			nodeBytes := contentBytes[start:end]
			result = append(result, h.applyDiffStylingToBytes(nodeBytes, int(start), diffMap)...)
		}

		lastEnd = end
	}

	// Append any remaining content
	if lastEnd < uint32(len(contentBytes)) {
		remaining := contentBytes[lastEnd:]
		result = append(result, h.applyDiffStylingToBytes(remaining, int(lastEnd), diffMap)...)
	}

	return string(result)
}

func (h *Highlighter) applyDiffStylingToBytes(content []byte, startPos int, diffMap map[int]diffmatchpatch.Operation) []byte {
	if len(content) == 0 {
		return content
	}

	var result []byte
	backgroundStyle := lipgloss.NewStyle().Background(lipgloss.Color("16"))

	i := 0
	for i < len(content) {
		pos := startPos + i
		if diffType, exists := diffMap[pos]; exists && (diffType == diffmatchpatch.DiffDelete || diffType == diffmatchpatch.DiffInsert) {
			// Find the end of this diff segment
			segmentStart := i
			for i < len(content) {
				pos = startPos + i
				if diffType, exists := diffMap[pos]; !exists || (diffType != diffmatchpatch.DiffDelete && diffType != diffmatchpatch.DiffInsert) {
					break
				}
				i++
			}
			// Apply background styling to this segment
			segment := content[segmentStart:i]
			styledSegment := backgroundStyle.Render(string(segment))
			result = append(result, []byte(styledSegment)...)
		} else {
			result = append(result, content[i])
			i++
		}
	}

	return result
}

func (h *Highlighter) applyCombinedStyling(text string, startPos int, syntaxStyle *lipgloss.Style, diffMap map[int]diffmatchpatch.Operation) string {
	var result strings.Builder

	i := 0
	for i < len(text) {
		pos := startPos + i
		if diffType, exists := diffMap[pos]; exists && (diffType == diffmatchpatch.DiffDelete || diffType == diffmatchpatch.DiffInsert) {
			// Find the end of this diff segment within the text
			segmentStart := i
			for i < len(text) {
				pos = startPos + i
				if diffType, exists := diffMap[pos]; !exists || (diffType != diffmatchpatch.DiffDelete && diffType != diffmatchpatch.DiffInsert) {
					break
				}
				i++
			}
			// Apply combined styling: syntax foreground + diff background
			segment := text[segmentStart:i]
			combinedStyle := syntaxStyle.Copy().Background(lipgloss.Color("16"))
			styledSegment := combinedStyle.Render(segment)
			result.WriteString(styledSegment)
		} else {
			// Find the end of this non-diff segment
			segmentStart := i
			for i < len(text) {
				pos = startPos + i
				if diffType, exists := diffMap[pos]; exists && (diffType == diffmatchpatch.DiffDelete || diffType == diffmatchpatch.DiffInsert) {
					break
				}
				i++
			}
			// Apply only syntax styling
			segment := text[segmentStart:i]
			styledSegment := syntaxStyle.Render(segment)
			result.WriteString(styledSegment)
		}
	}

	return result.String()
}
