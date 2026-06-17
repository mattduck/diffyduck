package highlight

import (
	"github.com/mattduck/diffyduck/pkg/highlight/grammars/elisp"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func ElispLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "elisp",
		Extensions: []string{".el"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(elisp.GetLanguage())
		},
		HighlightQuery: elispHighlightQuery,
	}
}
