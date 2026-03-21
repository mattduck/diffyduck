package highlight

import (
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
)

func PHPLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:         "php",
		Extensions:   []string{".php"},
		Interpreters: []string{"php"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_php.LanguagePHP()))
		},
		HighlightQuery: phpHighlightQuery,
	}
}
