package highlight

import (
	"github.com/mattduck/diffyduck/pkg/highlight/grammars/diff"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
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
