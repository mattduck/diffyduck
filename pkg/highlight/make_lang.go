package highlight

import (
	"unsafe"

	tree_sitter_make "github.com/tree-sitter-grammars/tree-sitter-make/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func MakeLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "make",
		Extensions: []string{".mk"},
		Filenames:  []string{"Makefile", "GNUmakefile", "makefile"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_make.Language()))
		},
		HighlightQuery: makeHighlightQuery,
	}
}
