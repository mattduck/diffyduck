package highlight

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/user/diffyduck/pkg/highlight/grammars/ini"
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
