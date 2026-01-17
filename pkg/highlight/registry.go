package highlight

import (
	"path/filepath"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ContextRule defines how to categorize a node based on its context.
// Used for ambiguous node types like "identifier" that could be
// a function name, variable, type, etc. depending on parent context.
type ContextRule struct {
	NodeType   string   // The node type this rule applies to
	ParentType string   // Required parent node type
	Field      string   // Optional: the field name in the parent
	Category   Category // Category to assign when rule matches
}

// LanguageConfig defines the tree-sitter configuration for a language.
type LanguageConfig struct {
	Name         string                       // Language name (e.g., "go", "javascript")
	Extensions   []string                     // File extensions (e.g., ".go", ".js")
	Language     func() *tree_sitter.Language // Returns the tree-sitter language
	NodeTypes    map[string]Category          // Direct node type -> category mappings
	ContextRules []ContextRule                // Context-dependent categorization rules
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

// Categorize determines the category for a node using the language config.
// First checks direct NodeTypes mapping, then tries ContextRules.
func (cfg *LanguageConfig) Categorize(node *tree_sitter.Node) Category {
	nodeType := node.Kind()

	// First, try direct mapping
	if cat, ok := cfg.NodeTypes[nodeType]; ok {
		return cat
	}

	// Then, try context rules
	parent := node.Parent()
	if parent == nil {
		return CategoryNone
	}

	parentType := parent.Kind()
	for _, rule := range cfg.ContextRules {
		if rule.NodeType != nodeType {
			continue
		}
		if rule.ParentType != parentType {
			continue
		}
		if rule.Field != "" {
			// Check if node is in the expected field
			fieldNode := parent.ChildByFieldName(rule.Field)
			if fieldNode == nil || !nodesEqual(fieldNode, node) {
				continue
			}
		}
		return rule.Category
	}

	return CategoryNone
}

// nodesEqual checks if two nodes represent the same syntax node.
func nodesEqual(a, b *tree_sitter.Node) bool {
	return a.StartByte() == b.StartByte() && a.EndByte() == b.EndByte()
}
