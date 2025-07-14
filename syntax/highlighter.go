package syntax

import (
	"path/filepath"
	"strings"

	"diffyduck/syntax/languages"
	"diffyduck/types"
	"github.com/charmbracelet/lipgloss"
	"github.com/sergi/go-diff/diffmatchpatch"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type Highlighter struct {
	parsers       map[string]*tree_sitter.Parser
	languages     map[string]LanguageDefinition
	extensionMap  map[string]string // file extension -> language name
	keywordStyle  lipgloss.Style
	stringStyle   lipgloss.Style
	commentStyle  lipgloss.Style
	constantStyle lipgloss.Style
}

func NewHighlighter() *Highlighter {
	keywordStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("4")) // blue

	stringStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")) // gray

	commentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")) // gray

	constantStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("3")).
		Bold(true) // bold yellow

	h := &Highlighter{
		parsers:       make(map[string]*tree_sitter.Parser),
		languages:     make(map[string]LanguageDefinition),
		extensionMap:  make(map[string]string),
		keywordStyle:  keywordStyle,
		stringStyle:   stringStyle,
		commentStyle:  commentStyle,
		constantStyle: constantStyle,
	}

	// Register Go language by default
	h.RegisterLanguage(languages.NewGoLanguage())

	return h
}

func (h *Highlighter) Close() {
	for _, parser := range h.parsers {
		if parser != nil {
			parser.Close()
		}
	}
	h.parsers = make(map[string]*tree_sitter.Parser)
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
	return h.highlightNode(content, root, lang)
}

func (h *Highlighter) highlightNode(content string, node *tree_sitter.Node, lang LanguageDefinition) string {
	contentBytes := []byte(content)
	result := make([]byte, 0, len(contentBytes)*2) // pre-allocate with extra space for ANSI codes
	lastEnd := uint32(0)

	h.walkNodes(node, contentBytes, &result, &lastEnd, lang)

	// Append any remaining content
	if lastEnd < uint32(len(contentBytes)) {
		result = append(result, contentBytes[lastEnd:]...)
	}

	return string(result)
}

func (h *Highlighter) walkNodes(node *tree_sitter.Node, content []byte, result *[]byte, lastEnd *uint32, lang LanguageDefinition) {
	if node == nil {
		return
	}

	nodeKind := node.Kind()
	start := uint32(node.StartByte())
	end := uint32(node.EndByte())

	// Check what type of node this is and apply appropriate styling
	var style *lipgloss.Style

	// Check if it's a keyword
	for _, keyword := range lang.GetKeywordNodeTypes() {
		if nodeKind == keyword {
			style = &h.keywordStyle
			break
		}
	}

	// Check if it's a comment
	if style == nil {
		for _, comment := range lang.GetCommentNodeTypes() {
			if nodeKind == comment {
				style = &h.commentStyle
				break
			}
		}
	}

	// Check if it's a string
	if style == nil {
		for _, str := range lang.GetStringNodeTypes() {
			if nodeKind == str {
				style = &h.stringStyle
				break
			}
		}
	}

	// Check if it's a literal
	if style == nil {
		for _, literal := range lang.GetLiteralNodeTypes() {
			if nodeKind == literal {
				style = &h.constantStyle
				break
			}
		}
	}

	// Apply styling if we found a match
	if style != nil && start >= *lastEnd && end <= uint32(len(content)) {
		// Add content before this node
		*result = append(*result, content[*lastEnd:start]...)

		// Get the text of this node
		nodeText := string(content[start:end])

		// Apply styling
		styledText := style.Render(nodeText)
		*result = append(*result, []byte(styledText)...)

		*lastEnd = end

		// For styled nodes, don't process children since we've handled the entire node
		return
	}

	// Recursively process child nodes
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		h.walkNodes(child, content, result, lastEnd, lang)
	}
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
	return h.highlightNodeWithWordDiff(content, root, lang, segments)
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

func (h *Highlighter) highlightNodeWithWordDiff(content string, node *tree_sitter.Node, lang LanguageDefinition, segments []types.DiffSegment) string {
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

	contentBytes := []byte(content)
	result := make([]byte, 0, len(contentBytes)*3) // pre-allocate with extra space for ANSI codes
	lastEnd := uint32(0)

	h.walkNodesWithWordDiff(node, contentBytes, &result, &lastEnd, lang, diffMap)

	// Append any remaining content
	if lastEnd < uint32(len(contentBytes)) {
		remaining := contentBytes[lastEnd:]
		result = append(result, h.applyDiffStylingToBytes(remaining, int(lastEnd), diffMap)...)
	}

	return string(result)
}

func (h *Highlighter) walkNodesWithWordDiff(node *tree_sitter.Node, content []byte, result *[]byte, lastEnd *uint32, lang LanguageDefinition, diffMap map[int]diffmatchpatch.Operation) {
	if node == nil {
		return
	}

	nodeKind := node.Kind()
	start := uint32(node.StartByte())
	end := uint32(node.EndByte())

	// Check what type of node this is and apply appropriate styling
	var style *lipgloss.Style

	// Check if it's a keyword
	for _, keyword := range lang.GetKeywordNodeTypes() {
		if nodeKind == keyword {
			style = &h.keywordStyle
			break
		}
	}

	// Check if it's a comment
	if style == nil {
		for _, comment := range lang.GetCommentNodeTypes() {
			if nodeKind == comment {
				style = &h.commentStyle
				break
			}
		}
	}

	// Check if it's a string
	if style == nil {
		for _, str := range lang.GetStringNodeTypes() {
			if nodeKind == str {
				style = &h.stringStyle
				break
			}
		}
	}

	// Check if it's a literal
	if style == nil {
		for _, literal := range lang.GetLiteralNodeTypes() {
			if nodeKind == literal {
				style = &h.constantStyle
				break
			}
		}
	}

	// Apply styling if we found a match
	if style != nil && start >= *lastEnd && end <= uint32(len(content)) {
		// Add content before this node
		beforeContent := content[*lastEnd:start]
		*result = append(*result, h.applyDiffStylingToBytes(beforeContent, int(*lastEnd), diffMap)...)

		// Get the text of this node
		nodeText := string(content[start:end])

		// Apply combined styling (syntax + word diff)
		styledText := h.applyCombinedStyling(nodeText, int(start), style, diffMap)
		*result = append(*result, []byte(styledText)...)

		*lastEnd = end

		// For styled nodes, don't process children since we've handled the entire node
		return
	}

	// Recursively process child nodes
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		h.walkNodesWithWordDiff(child, content, result, lastEnd, lang, diffMap)
	}
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
