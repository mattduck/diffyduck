package highlight

import (
	"unsafe"

	tree_sitter_toml "github.com/tree-sitter-grammars/tree-sitter-toml/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// TOMLLanguage returns the tree-sitter configuration for TOML.
func TOMLLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "toml",
		Extensions: []string{".toml"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_toml.Language()))
		},
		HighlightQuery: tomlHighlightQuery,
	}
}
