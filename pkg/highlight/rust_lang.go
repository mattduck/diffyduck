package highlight

import (
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_rust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
)

func RustLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "rust",
		Extensions: []string{".rs"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_rust.Language()))
		},
		HighlightQuery: rustHighlightQuery,
	}
}
