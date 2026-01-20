package highlight

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/user/diffyduck/pkg/highlight/grammars/sql"
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
