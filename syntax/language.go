package syntax

import (
	"unsafe"
)

// LanguageDefinition defines the interface for language-specific syntax highlighting
type LanguageDefinition interface {
	// GetLanguage returns the Tree-sitter language pointer for this language
	GetLanguage() unsafe.Pointer
	
	// GetKeywordNodeTypes returns node types that should be highlighted as keywords
	GetKeywordNodeTypes() []string
	
	// GetStringNodeTypes returns node types that represent string literals
	GetStringNodeTypes() []string
	
	// GetCommentNodeTypes returns node types that represent comments
	GetCommentNodeTypes() []string
	
	// GetLiteralNodeTypes returns node types that represent literal constants (nil, true, numbers, etc.)
	GetLiteralNodeTypes() []string
	
	// GetFileExtensions returns file extensions this language handles (e.g., [".go", ".mod"])
	GetFileExtensions() []string
	
	// GetLanguageName returns the human-readable name of this language
	GetLanguageName() string
}