package highlight

import (
	"github.com/mattduck/diffyduck/pkg/highlight/grammars/sql"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func SQLLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "sql",
		Extensions: []string{".sql"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(sql.GetLanguage())
		},
		HighlightQuery: sqlHighlightQuery,
	}
}
