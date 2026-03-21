package highlight

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/user/diffyduck/pkg/highlight/grammars/dockerfile"
)

func DockerfileLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "dockerfile",
		Extensions: []string{},
		Filenames:  []string{"Dockerfile", "Containerfile"},
		FilenamePredicate: func(basename string) bool {
			lower := strings.ToLower(basename)
			// Matches Dockerfile.prod, api.Dockerfile, containerfile.dev, etc.
			return strings.Contains(lower, "dockerfile") || strings.Contains(lower, "containerfile")
		},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(dockerfile.GetLanguage())
		},
		HighlightQuery: dockerfileHighlightQuery,
	}
}
