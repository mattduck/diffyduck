package highlight

import (
	"github.com/mattduck/diffyduck/pkg/highlight/grammars/ini"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func INILanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "ini",
		Extensions: []string{".ini", ".cfg", ".conf"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(ini.GetLanguage())
		},
		HighlightQuery: iniHighlightQuery,
	}
}
