package highlight

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/user/diffyduck/pkg/highlight/grammars/diff"
)

func DiffLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "diff",
		Extensions: []string{".diff", ".patch"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(diff.GetLanguage())
		},
		HighlightQuery: diffHighlightQuery,
	}
}
