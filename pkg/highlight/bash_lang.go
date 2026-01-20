package highlight

import (
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_bash "github.com/tree-sitter/tree-sitter-bash/bindings/go"
)

func BashLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "bash",
		Extensions: []string{".sh", ".bash", ".zsh"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_bash.Language()))
		},
		HighlightQuery: bashHighlightQuery,
	}
}
