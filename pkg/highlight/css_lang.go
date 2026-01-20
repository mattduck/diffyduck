package highlight

import (
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_css "github.com/tree-sitter/tree-sitter-css/bindings/go"
)

func CSSLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "css",
		Extensions: []string{".css"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_css.Language()))
		},
		HighlightQuery: cssHighlightQuery,
	}
}
