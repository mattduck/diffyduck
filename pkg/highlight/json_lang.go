package highlight

import (
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_json "github.com/tree-sitter/tree-sitter-json/bindings/go"
)

func JSONLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "json",
		Extensions: []string{".json"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_json.Language()))
		},
		HighlightQuery: jsonHighlightQuery,
	}
}
