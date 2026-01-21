package structure

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// Extractor extracts structural elements from tree-sitter parse trees.
type Extractor struct {
	// Language-specific extractors
	extractors map[string]languageExtractor
}

// languageExtractor defines how to extract structure for a specific language.
type languageExtractor interface {
	// Extract extracts structural entries from the given tree.
	Extract(tree *tree_sitter.Tree, content []byte) []Entry
}

// NewExtractor creates a new structure extractor.
func NewExtractor() *Extractor {
	return &Extractor{
		extractors: map[string]languageExtractor{
			"go": &goExtractor{},
		},
	}
}

// Extract extracts structural elements from the given tree for the specified language.
// Returns nil if the language is not supported.
func (e *Extractor) Extract(tree *tree_sitter.Tree, content []byte, lang string) *Map {
	extractor, ok := e.extractors[lang]
	if !ok {
		return nil
	}

	entries := extractor.Extract(tree, content)
	if len(entries) == 0 {
		return nil
	}

	return NewMap(entries)
}

// SupportsLanguage returns true if the extractor supports the given language.
func (e *Extractor) SupportsLanguage(lang string) bool {
	_, ok := e.extractors[lang]
	return ok
}
