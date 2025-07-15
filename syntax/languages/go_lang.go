package languages

import (
	_ "embed"
	"unsafe"

	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

//go:embed go_highlights.scm
var goHighlightQuery string

// GoLanguage implements syntax highlighting for the Go programming language
type GoLanguage struct{}

// NewGoLanguage creates a new Go language definition
func NewGoLanguage() *GoLanguage {
	return &GoLanguage{}
}

func (g *GoLanguage) GetLanguage() unsafe.Pointer {
	return tree_sitter_go.Language()
}

func (g *GoLanguage) GetHighlightQuery() string {
	return goHighlightQuery
}

func (g *GoLanguage) GetFileExtensions() []string {
	return []string{".go", ".mod"}
}

func (g *GoLanguage) GetLanguageName() string {
	return "Go"
}

// Note: GoLanguage implements the LanguageDefinition interface defined in the parent syntax package
