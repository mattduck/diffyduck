package highlight

import (
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
)

func JavaScriptLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "javascript",
		Extensions: []string{".js", ".jsx", ".mjs", ".cjs"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_javascript.Language()))
		},
		HighlightQuery: javascriptHighlightQuery,
	}
}
