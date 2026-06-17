package highlight

import (
	"github.com/mattduck/diffyduck/pkg/highlight/grammars/org"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
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
