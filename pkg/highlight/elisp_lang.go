package highlight

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/user/diffyduck/pkg/highlight/grammars/elisp"
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
