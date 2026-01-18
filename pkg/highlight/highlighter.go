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

	// Cached parsers and queries per language (not thread-safe)
	mu      sync.Mutex
	parsers map[string]*tree_sitter.Parser
	queries map[string]*tree_sitter.Query
}

// New creates a new Highlighter with the default theme.
func New() *Highlighter {
	return &Highlighter{
		theme:    DefaultTheme(),
		registry: NewRegistry(),
		parsers:  make(map[string]*tree_sitter.Parser),
		queries:  make(map[string]*tree_sitter.Query),
	}
}

// NewWithTheme creates a new Highlighter with a custom theme.
func NewWithTheme(theme Theme) *Highlighter {
	return &Highlighter{
		theme:    theme,
		registry: NewRegistry(),
		parsers:  make(map[string]*tree_sitter.Parser),
		queries:  make(map[string]*tree_sitter.Query),
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

	parser, query := h.getParserAndQuery(cfg)
	if parser == nil || query == nil {
		return nil, nil
	}

	h.mu.Lock()
	tree := parser.Parse(content, nil)
	h.mu.Unlock()

	if tree == nil {
		return nil, nil
	}
	defer tree.Close()

	spans := h.runQuery(tree.RootNode(), query, content)

	// Sort by start position
	sort.Slice(spans, func(i, j int) bool {
		return spans[i].Start < spans[j].Start
	})

	return spans, nil
}

// getParserAndQuery returns cached parser and query for the language config.
func (h *Highlighter) getParserAndQuery(cfg *LanguageConfig) (*tree_sitter.Parser, *tree_sitter.Query) {
	h.mu.Lock()
	defer h.mu.Unlock()

	parser, parserOk := h.parsers[cfg.Name]
	query, queryOk := h.queries[cfg.Name]

	if parserOk && queryOk {
		return parser, query
	}

	// Create parser if needed
	if !parserOk {
		parser = tree_sitter.NewParser()
		lang := cfg.Language()
		if err := parser.SetLanguage(lang); err != nil {
			parser.Close()
			return nil, nil
		}
		h.parsers[cfg.Name] = parser

		// Create query (requires language)
		var qErr *tree_sitter.QueryError
		query, qErr = tree_sitter.NewQuery(lang, cfg.HighlightQuery)
		if qErr != nil {
			// Query compilation failed - log would be nice but just skip highlighting
			return nil, nil
		}
		h.queries[cfg.Name] = query
	}

	return parser, query
}

// runQuery executes the highlight query and collects spans.
func (h *Highlighter) runQuery(node *tree_sitter.Node, query *tree_sitter.Query, content []byte) []Span {
	var spans []Span

	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()

	captures := cursor.Captures(query, node, content)
	captureNames := query.CaptureNames()

	for {
		match, captureIdx := captures.Next()
		if match == nil {
			break
		}

		// captureIdx is the index into match.Captures for the current capture
		if captureIdx >= uint(len(match.Captures)) {
			continue
		}

		capture := match.Captures[captureIdx]
		captureName := captureNames[capture.Index]
		cat := CategoryForCapture(captureName)
		if cat == CategoryNone {
			continue
		}

		spans = append(spans, Span{
			Start:    int(capture.Node.StartByte()),
			End:      int(capture.Node.EndByte()),
			Category: cat,
		})
	}

	return spans
}

// Close releases all resources held by the highlighter.
func (h *Highlighter) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, parser := range h.parsers {
		parser.Close()
	}
	for _, query := range h.queries {
		query.Close()
	}
	h.parsers = nil
	h.queries = nil
}

// SupportsFile returns true if the highlighter can highlight the given file.
func (h *Highlighter) SupportsFile(filename string) bool {
	return h.registry.ForFile(filename) != nil
}
