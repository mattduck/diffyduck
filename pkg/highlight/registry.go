package highlight

import (
	"bytes"
	"path/filepath"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// LanguageConfig defines the tree-sitter configuration for a language.
type LanguageConfig struct {
	Name              string                       // Language name (e.g., "go", "python")
	Extensions        []string                     // File extensions (e.g., ".go", ".py")
	Filenames         []string                     // Exact filenames (e.g., "Makefile", "Dockerfile")
	FilenamePredicate func(basename string) bool   // Optional match on basename (e.g., contains "Dockerfile")
	Interpreters      []string                     // Shebang interpreter names (e.g., "bash", "python3")
	Language          func() *tree_sitter.Language // Returns the tree-sitter language
	HighlightQuery    string                       // The highlight query (.scm format)
}

// Registry holds all registered language configurations.
type Registry struct {
	byExtension   map[string]*LanguageConfig
	byFilename    map[string]*LanguageConfig
	byName        map[string]*LanguageConfig
	byInterpreter map[string]*LanguageConfig
	predicates    []*LanguageConfig // configs with FilenamePredicate set
}

// NewRegistry creates a new language registry with built-in languages.
func NewRegistry() *Registry {
	r := &Registry{
		byExtension:   make(map[string]*LanguageConfig),
		byFilename:    make(map[string]*LanguageConfig),
		byName:        make(map[string]*LanguageConfig),
		byInterpreter: make(map[string]*LanguageConfig),
	}

	// Register built-in languages (alphabetical order)
	r.Register(BashLanguage())
	r.Register(CLanguage())
	r.Register(CSSLanguage())
	r.Register(DiffLanguage())
	r.Register(DockerfileLanguage())
	r.Register(ElispLanguage())
	r.Register(GoLanguage())
	r.Register(GraphQLLanguage())
	r.Register(HTMLLanguage())
	r.Register(INILanguage())
	r.Register(JavaScriptLanguage())
	r.Register(JSONLanguage())
	r.Register(MakeLanguage())
	r.Register(MarkdownLanguage())
	r.Register(OrgLanguage())
	r.Register(PHPLanguage())
	r.Register(PythonLanguage())
	r.Register(RustLanguage())
	r.Register(SQLLanguage())
	r.Register(TOMLLanguage())
	r.Register(TypeScriptLanguage())
	r.Register(XMLLanguage())
	r.Register(YAMLLanguage())

	return r
}

// Register adds a language configuration to the registry.
func (r *Registry) Register(cfg *LanguageConfig) {
	r.byName[cfg.Name] = cfg
	for _, ext := range cfg.Extensions {
		r.byExtension[ext] = cfg
	}
	for _, name := range cfg.Filenames {
		r.byFilename[name] = cfg
	}
	for _, interp := range cfg.Interpreters {
		r.byInterpreter[interp] = cfg
	}
	if cfg.FilenamePredicate != nil {
		r.predicates = append(r.predicates, cfg)
	}
}

// ForFile returns the language configuration for a filename, or nil if unknown.
func (r *Registry) ForFile(filename string) *LanguageConfig {
	// Check exact filename match first (e.g., "Makefile", "Dockerfile")
	base := filepath.Base(filename)
	if cfg := r.byFilename[base]; cfg != nil {
		return cfg
	}
	// Fall back to extension match
	ext := strings.ToLower(filepath.Ext(filename))
	if cfg := r.byExtension[ext]; cfg != nil {
		return cfg
	}
	// Fall back to predicate match (e.g., basename contains "Dockerfile")
	for _, cfg := range r.predicates {
		if cfg.FilenamePredicate(base) {
			return cfg
		}
	}
	return nil
}

// ForFileWithContent returns the language configuration for a filename,
// falling back to shebang detection if the extension/filename is unknown.
func (r *Registry) ForFileWithContent(filename string, content []byte) *LanguageConfig {
	if cfg := r.ForFile(filename); cfg != nil {
		return cfg
	}
	return r.forShebang(content)
}

// forShebang checks the first line of content for a shebang (#!) and returns
// the matching language configuration, or nil if not found.
func (r *Registry) forShebang(content []byte) *LanguageConfig {
	if len(content) < 3 || content[0] != '#' || content[1] != '!' {
		return nil
	}

	// Extract first line
	firstLine := content
	if i := bytes.IndexByte(content, '\n'); i >= 0 {
		firstLine = content[:i]
	}

	// Parse interpreter from shebang:
	//   #!/usr/bin/env python3  → "python3"
	//   #!/bin/bash             → "bash"
	//   #!/usr/bin/perl -w      → "perl"
	line := strings.TrimSpace(string(firstLine[2:])) // strip "#!"
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil
	}

	// If invoked via "env", the interpreter is the next argument
	// (skip env flags like -S)
	cmd := filepath.Base(parts[0])
	if cmd == "env" {
		for _, arg := range parts[1:] {
			if !strings.HasPrefix(arg, "-") {
				cmd = filepath.Base(arg)
				break
			}
		}
		if cmd == "env" {
			return nil // bare "env" with no interpreter
		}
	}

	return r.byInterpreter[cmd]
}

// ForName returns the language configuration by name, or nil if unknown.
func (r *Registry) ForName(name string) *LanguageConfig {
	return r.byName[name]
}

// captureToCategory maps tree-sitter capture names to our highlight categories.
//
// Most captures follow standard conventions (keyword, string, comment, etc.) used
// by nvim-treesitter and other editors. Hierarchical names like "keyword.type"
// fall back to their prefix ("keyword") if not explicitly mapped.
//
// Some grammars (notably org-mode) use non-standard capture names. Rather than
// rewriting their queries, we add those custom names here. See the "Org-mode
// specific captures" section below.
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

	// Org-mode specific captures (from tree-sitter-org)
	"OrgStars1":               CategoryKeyword,
	"OrgStars2":               CategoryKeyword,
	"OrgStars3":               CategoryKeyword,
	"OrgKeywordTodo":          CategoryConstant,
	"OrgKeywordDone":          CategoryString,
	"OrgPriority":             CategoryConstant,
	"OrgPriorityCookie":       CategoryConstant,
	"OrgTag":                  CategoryTag,
	"OrgTagList":              CategoryTag,
	"OrgPropertyDrawer":       CategoryComment,
	"OrgPropertyName":         CategoryField,
	"OrgPropertyValue":        CategoryString,
	"OrgProperty":             CategoryComment,
	"OrgTimestampInactive":    CategoryString,
	"OrgTimestampActive":      CategoryString,
	"OrgTimestampDay":         CategoryString,
	"OrgTimestampDate":        CategoryString,
	"OrgTimestampTime":        CategoryString,
	"OrgTimestampRepeat":      CategoryConstant,
	"OrgTimestampDelay":       CategoryConstant,
	"OrgFootnoteLabel":        CategoryTag,
	"OrgFootnoteDescription":  CategoryComment,
	"OrgFootnoteDefinition":   CategoryComment,
	"OrgDirectiveName":        CategoryKeyword,
	"OrgDirectiveValue":       CategoryString,
	"OrgDirective":            CategoryKeyword,
	"OrgComment":              CategoryComment,
	"OrgDrawerName":           CategoryKeyword,
	"OrgDrawerContents":       CategoryComment,
	"OrgDrawer":               CategoryComment,
	"OrgBlockName":            CategoryKeyword,
	"OrgBlockContents":        CategoryString,
	"OrgBlock":                CategoryString,
	"OrgDynamicBlockName":     CategoryKeyword,
	"OrgDynamicBlockContents": CategoryComment,
	"OrgDynamicBlock":         CategoryComment,
	"OrgListBullet":           CategoryKeyword,
	"OrgCheckbox":             CategoryKeyword,
	"OrgCheckInProgress":      CategoryConstant,
	"OrgCheckDone":            CategoryString,
	"OrgTableHorizontalRuler": CategoryPunctuation,
	"OrgCellFormula":          CategoryNumber,
	"OrgCellNumber":           CategoryNumber,
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
