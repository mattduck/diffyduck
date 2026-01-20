package highlight

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/user/diffyduck/pkg/highlight/grammars/graphql"
)

func GraphQLLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "graphql",
		Extensions: []string{".graphql", ".gql"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(graphql.GetLanguage())
		},
		HighlightQuery: graphqlHighlightQuery,
	}
}
