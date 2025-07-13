package languages

import (
	"unsafe"
	
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

// GoLanguage implements syntax highlighting for the Go programming language
type GoLanguage struct{}

// NewGoLanguage creates a new Go language definition
func NewGoLanguage() *GoLanguage {
	return &GoLanguage{}
}

func (g *GoLanguage) GetLanguage() unsafe.Pointer {
	return tree_sitter_go.Language()
}

func (g *GoLanguage) GetKeywordNodeTypes() []string {
	return []string{
		"break", "case", "chan", "const", "continue", "default", "defer",
		"else", "fallthrough", "for", "func", "go", "goto", "if", "import",
		"interface", "map", "package", "range", "return", "select", "struct",
		"switch", "type", "var",
	}
}

func (g *GoLanguage) GetStringNodeTypes() []string {
	return []string{
		"interpreted_string_literal",
		"raw_string_literal", 
		"rune_literal",
	}
}

func (g *GoLanguage) GetCommentNodeTypes() []string {
	return []string{
		"comment",
	}
}

func (g *GoLanguage) GetLiteralNodeTypes() []string {
	return []string{
		"nil", "true", "false",
		"int_literal", "float_literal",
	}
}

func (g *GoLanguage) GetFileExtensions() []string {
	return []string{".go", ".mod"}
}

func (g *GoLanguage) GetLanguageName() string {
	return "Go"
}

// Note: GoLanguage implements the LanguageDefinition interface defined in the parent syntax package