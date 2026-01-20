package highlight

import (
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

func TypeScriptLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "typescript",
		Extensions: []string{".ts", ".tsx"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_typescript.LanguageTypescript()))
		},
		HighlightQuery: typescriptHighlightQuery,
	}
}
