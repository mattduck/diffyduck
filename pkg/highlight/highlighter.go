package highlight

import (
	"sort"
	"sync"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// Highlighter provides syntax highlighting for source code.
type Highlighter struct {
	theme    Theme
	registry *Registry

	// Cached parsers per language (parsers are not thread-safe)
	mu      sync.Mutex
	parsers map[string]*tree_sitter.Parser
}

// New creates a new Highlighter with the default theme.
func New() *Highlighter {
	return &Highlighter{
		theme:    DefaultTheme(),
		registry: NewRegistry(),
		parsers:  make(map[string]*tree_sitter.Parser),
	}
}

// NewWithTheme creates a new Highlighter with a custom theme.
func NewWithTheme(theme Theme) *Highlighter {
	return &Highlighter{
		theme:    theme,
		registry: NewRegistry(),
		parsers:  make(map[string]*tree_sitter.Parser),
	}
}

// Theme returns the highlighter's theme.
func (h *Highlighter) Theme() Theme {
	return h.theme
}

// Highlight returns spans for the given source code.
// Returns nil if the language is not supported.
func (h *Highlighter) Highlight(filename string, content []byte) ([]Span, error) {
	cfg := h.registry.ForFile(filename)
	if cfg == nil {
		return nil, nil // Unknown language, no highlighting
	}

	parser := h.getParser(cfg)
	if parser == nil {
		return nil, nil
	}

	h.mu.Lock()
	tree := parser.Parse(content, nil)
	h.mu.Unlock()

	if tree == nil {
		return nil, nil
	}
	defer tree.Close()

	spans := h.walkTree(tree.RootNode(), cfg)

	// Sort by start position
	sort.Slice(spans, func(i, j int) bool {
		return spans[i].Start < spans[j].Start
	})

	return spans, nil
}

// getParser returns a cached parser for the language config.
func (h *Highlighter) getParser(cfg *LanguageConfig) *tree_sitter.Parser {
	h.mu.Lock()
	defer h.mu.Unlock()

	if parser, ok := h.parsers[cfg.Name]; ok {
		return parser
	}

	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(cfg.Language()); err != nil {
		parser.Close()
		return nil
	}
	h.parsers[cfg.Name] = parser
	return parser
}

// walkTree recursively walks the syntax tree and collects spans.
func (h *Highlighter) walkTree(node *tree_sitter.Node, cfg *LanguageConfig) []Span {
	var spans []Span

	cursor := node.Walk()
	defer cursor.Close()

	h.walkCursor(cursor, cfg, &spans)
	return spans
}

// walkCursor is the recursive helper for tree walking.
func (h *Highlighter) walkCursor(cursor *tree_sitter.TreeCursor, cfg *LanguageConfig, spans *[]Span) {
	node := cursor.Node()

	// Get category for this node
	cat := cfg.Categorize(node)
	if cat != CategoryNone {
		*spans = append(*spans, Span{
			Start:    int(node.StartByte()),
			End:      int(node.EndByte()),
			Category: cat,
		})
	}

	// Visit children
	if cursor.GotoFirstChild() {
		for {
			h.walkCursor(cursor, cfg, spans)
			if !cursor.GotoNextSibling() {
				break
			}
		}
		cursor.GotoParent()
	}
}

// Close releases all resources held by the highlighter.
func (h *Highlighter) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, parser := range h.parsers {
		parser.Close()
	}
	h.parsers = nil
}

// SupportsFile returns true if the highlighter can highlight the given file.
func (h *Highlighter) SupportsFile(filename string) bool {
	return h.registry.ForFile(filename) != nil
}
