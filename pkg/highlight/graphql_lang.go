package highlight

import (
	"github.com/mattduck/diffyduck/pkg/highlight/grammars/graphql"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
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
