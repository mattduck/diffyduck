package highlight

import (
	"path/filepath"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// LanguageConfig defines the tree-sitter configuration for a language.
type LanguageConfig struct {
	Name           string                       // Language name (e.g., "go", "python")
	Extensions     []string                     // File extensions (e.g., ".go", ".py")
	Language       func() *tree_sitter.Language // Returns the tree-sitter language
	HighlightQuery string                       // The highlight query (.scm format)
}

// Registry holds all registered language configurations.
type Registry struct {
	byExtension map[string]*LanguageConfig
	byName      map[string]*LanguageConfig
}

// NewRegistry creates a new language registry with built-in languages.
func NewRegistry() *Registry {
	r := &Registry{
		byExtension: make(map[string]*LanguageConfig),
		byName:      make(map[string]*LanguageConfig),
	}

	// Register built-in languages
	r.Register(GoLanguage())
	r.Register(PythonLanguage())
	r.Register(YAMLLanguage())
	r.Register(TOMLLanguage())

	return r
}

// Register adds a language configuration to the registry.
func (r *Registry) Register(cfg *LanguageConfig) {
	r.byName[cfg.Name] = cfg
	for _, ext := range cfg.Extensions {
		r.byExtension[ext] = cfg
	}
}

// ForFile returns the language configuration for a filename, or nil if unknown.
func (r *Registry) ForFile(filename string) *LanguageConfig {
	ext := strings.ToLower(filepath.Ext(filename))
	return r.byExtension[ext]
}

// ForName returns the language configuration by name, or nil if unknown.
func (r *Registry) ForName(name string) *LanguageConfig {
	return r.byName[name]
}

// captureToCategory maps standard tree-sitter capture names to our categories.
// This follows the conventions used by nvim-treesitter and other editors.
// Hierarchical names (e.g., "keyword.type") fall back to their prefix ("keyword")
// if not explicitly mapped - see CategoryForCapture.
var captureToCategory = map[string]Category{
	// Functions
	"function":             CategoryFunction,
	"function.builtin":     CategoryFunction,
	"function.method":      CategoryFunction,
	"function.call":        CategoryFunctionCall,
	"function.method.call": CategoryFunctionCall,
	"function.macro":       CategoryFunction,
	"method":               CategoryFunction,
	"constructor":          CategoryFunction,

	// Types
	"type":            CategoryType,
	"type.builtin":    CategoryType,
	"type.definition": CategoryType,

	// Variables and properties
	"variable":           CategoryVariable,
	"variable.builtin":   CategoryVariable,
	"variable.member":    CategoryField,
	"variable.parameter": CategoryParameter,
	"property":           CategoryField,
	"field":              CategoryField,
	"parameter":          CategoryParameter,

	// Constants - includes true, false, nil, None, etc.
	"constant":         CategoryConstant,
	"constant.builtin": CategoryConstant,
	"boolean":          CategoryBoolean,

	// Keywords
	"keyword":             CategoryKeyword,
	"keyword.control":     CategoryKeyword,
	"keyword.function":    CategoryKeyword,
	"keyword.operator":    CategoryKeyword,
	"keyword.return":      CategoryKeyword,
	"keyword.conditional": CategoryKeyword,
	"keyword.repeat":      CategoryKeyword,
	"keyword.import":      CategoryKeyword,
	"keyword.exception":   CategoryKeyword,

	// Operators
	"operator": CategoryOperator,

	// Punctuation
	"punctuation":           CategoryPunctuation,
	"punctuation.bracket":   CategoryPunctuation,
	"punctuation.delimiter": CategoryPunctuation,
	"punctuation.special":   CategoryPunctuation,

	// Strings
	"string":         CategoryString,
	"string.escape":  CategoryString,
	"string.special": CategoryString,
	"escape":         CategoryString,

	// Numbers
	"number":       CategoryNumber,
	"number.float": CategoryNumber,
	"float":        CategoryNumber,

	// Comments
	"comment":               CategoryComment,
	"comment.line":          CategoryComment,
	"comment.block":         CategoryComment,
	"comment.documentation": CategoryDocComment,

	// Attributes/decorators
	"attribute": CategoryAttribute,
	"decorator": CategoryAttribute,

	// Namespaces
	"namespace": CategoryNamespace,
	"module":    CategoryNamespace,

	// Tags (for struct tags, annotations, etc.)
	"tag":   CategoryTag,
	"label": CategoryTag,

	// Embedded content
	"embedded": CategoryNone,
}

// CategoryForCapture returns the category for a capture name.
// It handles both simple names like "keyword" and hierarchical names like "keyword.control".
func CategoryForCapture(name string) Category {
	// Try exact match first
	if cat, ok := captureToCategory[name]; ok {
		return cat
	}

	// Try prefix match (e.g., "keyword.something" → "keyword")
	if idx := strings.LastIndex(name, "."); idx != -1 {
		prefix := name[:idx]
		if cat, ok := captureToCategory[prefix]; ok {
			return cat
		}
	}

	return CategoryNone
}
