package highlight

import (
	"unsafe"

	tree_sitter_yaml "github.com/tree-sitter-grammars/tree-sitter-yaml/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// YAMLLanguage returns the tree-sitter configuration for YAML.
func YAMLLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "yaml",
		Extensions: []string{".yaml", ".yml"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_yaml.Language()))
		},
		HighlightQuery: yamlHighlightQuery,
	}
}
