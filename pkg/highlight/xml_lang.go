package highlight

import (
	"unsafe"

	tree_sitter_xml "github.com/tree-sitter-grammars/tree-sitter-xml/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func XMLLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "xml",
		Extensions: []string{".xml", ".xsl", ".xslt", ".svg"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_xml.LanguageXML()))
		},
		HighlightQuery: xmlHighlightQuery,
	}
}
