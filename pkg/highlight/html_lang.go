package highlight

import (
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_html "github.com/tree-sitter/tree-sitter-html/bindings/go"
)

func HTMLLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "html",
		Extensions: []string{".html", ".htm"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_html.Language()))
		},
		HighlightQuery: htmlHighlightQuery,
	}
}
