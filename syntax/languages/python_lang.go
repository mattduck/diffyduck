package languages

import (
	_ "embed"
	"unsafe"

	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
)

//go:embed python_highlights.scm
var pythonHighlightQuery string

// PythonLanguage implements syntax highlighting for the Python programming language
type PythonLanguage struct{}

// NewPythonLanguage creates a new Python language definition
func NewPythonLanguage() *PythonLanguage {
	return &PythonLanguage{}
}

func (p *PythonLanguage) GetLanguage() unsafe.Pointer {
	return tree_sitter_python.Language()
}

func (p *PythonLanguage) GetHighlightQuery() string {
	return pythonHighlightQuery
}

func (p *PythonLanguage) GetFileExtensions() []string {
	return []string{".py", ".pyw", ".pyi"}
}

func (p *PythonLanguage) GetLanguageName() string {
	return "Python"
}

// Note: PythonLanguage implements the LanguageDefinition interface defined in the parent syntax package
