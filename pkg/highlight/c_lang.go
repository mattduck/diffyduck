package highlight

import (
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_c "github.com/tree-sitter/tree-sitter-c/bindings/go"
)

func CLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "c",
		Extensions: []string{".c", ".h"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_c.Language()))
		},
		HighlightQuery: cHighlightQuery,
	}
}
