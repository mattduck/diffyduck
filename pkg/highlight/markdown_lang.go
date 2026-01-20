package highlight

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/user/diffyduck/pkg/highlight/grammars/markdown"
)

func MarkdownLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "markdown",
		Extensions: []string{".md", ".markdown"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(markdown.GetLanguage())
		},
		HighlightQuery: markdownHighlightQuery,
	}
}
