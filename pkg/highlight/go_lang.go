package highlight

import (
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

// GoLanguage returns the tree-sitter configuration for Go.
func GoLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "go",
		Extensions: []string{".go"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_go.Language()))
		},
		HighlightQuery: goHighlightQuery,
	}
}
