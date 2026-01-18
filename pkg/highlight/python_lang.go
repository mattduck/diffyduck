package highlight

import (
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
)

// PythonLanguage returns the tree-sitter configuration for Python.
func PythonLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "python",
		Extensions: []string{".py", ".pyi"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_python.Language()))
		},
		HighlightQuery: pythonHighlightQuery,
	}
}
