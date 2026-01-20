package highlight

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/user/diffyduck/pkg/highlight/grammars/org"
)

func OrgLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "org",
		Extensions: []string{".org"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(org.GetLanguage())
		},
		HighlightQuery: orgHighlightQuery,
	}
}
