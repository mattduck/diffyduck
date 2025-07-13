package syntax

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

type Highlighter struct {
	parser        *tree_sitter.Parser
	keywordStyle  lipgloss.Style
	stringStyle   lipgloss.Style
	commentStyle  lipgloss.Style
	constantStyle lipgloss.Style
}

// Go keywords to highlight
var goKeywords = map[string]bool{
	"break":       true,
	"case":        true,
	"chan":        true,
	"const":       true,
	"continue":    true,
	"default":     true,
	"defer":       true,
	"else":        true,
	"fallthrough": true,
	"for":         true,
	"func":        true,
	"go":          true,
	"goto":        true,
	"if":          true,
	"import":      true,
	"interface":   true,
	"map":         true,
	"package":     true,
	"range":       true,
	"return":      true,
	"select":      true,
	"struct":      true,
	"switch":      true,
	"type":        true,
	"var":         true,
}

func NewHighlighter() *Highlighter {
	parser := tree_sitter.NewParser()
	parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_go.Language()))
	
	keywordStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("4")) // blue
	
	stringStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")) // gray
	
	commentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")) // gray
	
	constantStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("3")).
		Bold(true) // bold yellow
	
	return &Highlighter{
		parser:        parser,
		keywordStyle:  keywordStyle,
		stringStyle:   stringStyle,
		commentStyle:  commentStyle,
		constantStyle: constantStyle,
	}
}

func (h *Highlighter) Close() {
	if h.parser != nil {
		h.parser.Close()
		h.parser = nil
	}
}

func (h *Highlighter) IsGoFile(filepath string) bool {
	return strings.HasSuffix(strings.ToLower(filepath), ".go")
}

func (h *Highlighter) HighlightLine(content, language string) string {
	if language != "go" || content == "" {
		return content
	}
	
	// Parse the line content
	tree := h.parser.Parse([]byte(content), nil)
	if tree == nil {
		return content // fallback on parse error
	}
	defer tree.Close()
	
	root := tree.RootNode()
	return h.highlightNode(content, root)
}

func (h *Highlighter) highlightNode(content string, node *tree_sitter.Node) string {
	contentBytes := []byte(content)
	result := make([]byte, 0, len(contentBytes)*2) // pre-allocate with extra space for ANSI codes
	lastEnd := uint32(0)
	
	h.walkNodes(node, contentBytes, &result, &lastEnd)
	
	// Append any remaining content
	if lastEnd < uint32(len(contentBytes)) {
		result = append(result, contentBytes[lastEnd:]...)
	}
	
	return string(result)
}

func (h *Highlighter) walkNodes(node *tree_sitter.Node, content []byte, result *[]byte, lastEnd *uint32) {
	if node == nil {
		return
	}
	
	nodeKind := node.Kind()
	start := uint32(node.StartByte())
	end := uint32(node.EndByte())
	
	// Check what type of node this is and apply appropriate styling
	var style *lipgloss.Style
	
	if goKeywords[nodeKind] {
		// Go keyword
		style = &h.keywordStyle
	} else if nodeKind == "comment" {
		// Comment (both // and /* */)
		style = &h.commentStyle
	} else if nodeKind == "interpreted_string_literal" || nodeKind == "raw_string_literal" || nodeKind == "rune_literal" {
		// String literals
		style = &h.stringStyle
	} else if nodeKind == "nil" || nodeKind == "true" || nodeKind == "false" || nodeKind == "int_literal" || nodeKind == "float_literal" {
		// Literal constants and numeric literals
		style = &h.constantStyle
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
		h.walkNodes(child, content, result, lastEnd)
	}
}