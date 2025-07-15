package syntax

import (
	"unsafe"
)

// LanguageDefinition defines the interface for language-specific syntax highlighting
type LanguageDefinition interface {
	// GetLanguage returns the Tree-sitter language pointer for this language
	GetLanguage() unsafe.Pointer

	// GetHighlightQuery returns the tree-sitter query string for syntax highlighting
	GetHighlightQuery() string

	// GetFileExtensions returns file extensions this language handles (e.g., [".go", ".mod"])
	GetFileExtensions() []string

	// GetLanguageName returns the human-readable name of this language
	GetLanguageName() string
}
