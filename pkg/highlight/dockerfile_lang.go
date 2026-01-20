package highlight

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/user/diffyduck/pkg/highlight/grammars/dockerfile"
)

func DockerfileLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "dockerfile",
		Extensions: []string{},
		Filenames:  []string{"Dockerfile", "Containerfile"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(dockerfile.GetLanguage())
		},
		HighlightQuery: dockerfileHighlightQuery,
	}
}
