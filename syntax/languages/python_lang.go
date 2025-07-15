package languages

import (
	"unsafe"

	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
)

// PythonLanguage implements syntax highlighting for the Python programming language
type PythonLanguage struct{}

// NewPythonLanguage creates a new Python language definition
func NewPythonLanguage() *PythonLanguage {
	return &PythonLanguage{}
}

func (p *PythonLanguage) GetLanguage() unsafe.Pointer {
	return tree_sitter_python.Language()
}

func (p *PythonLanguage) GetKeywordNodeTypes() []string {
	return []string{
		"False", "None", "True", "and", "as", "assert", "async", "await",
		"break", "class", "continue", "def", "del", "elif", "else", "except",
		"finally", "for", "from", "global", "if", "import", "in", "is",
		"lambda", "nonlocal", "not", "or", "pass", "raise", "return", "try",
		"while", "with", "yield",
	}
}

func (p *PythonLanguage) GetStringNodeTypes() []string {
	return []string{
		"string",
		"string_literal",
		"concatenated_string",
		"format_string",
		"f_string",
	}
}

func (p *PythonLanguage) GetCommentNodeTypes() []string {
	return []string{
		"comment",
	}
}

func (p *PythonLanguage) GetLiteralNodeTypes() []string {
	return []string{
		"integer", "float", "complex",
		"true", "false", "none",
		"ellipsis",
	}
}

func (p *PythonLanguage) GetFunctionDefinitionNodeTypes() []string {
	return []string{}
}

func (p *PythonLanguage) GetFunctionCallNodeTypes() []string {
	return []string{}
}

func (p *PythonLanguage) GetTypeNodeTypes() []string {
	return []string{
		"type",
	}
}

func (p *PythonLanguage) GetFileExtensions() []string {
	return []string{".py", ".pyw", ".pyi"}
}

func (p *PythonLanguage) GetLanguageName() string {
	return "Python"
}

// Note: PythonLanguage implements the LanguageDefinition interface defined in the parent syntax package
