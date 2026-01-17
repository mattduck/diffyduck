package highlight

import (
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

// GoLanguage returns the tree-sitter configuration for Go.
func GoLanguage() *LanguageConfig {
	return &LanguageConfig{
		Name:       "go",
		Extensions: []string{".go"},
		Language: func() *tree_sitter.Language {
			return tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_go.Language()))
		},
		NodeTypes: map[string]Category{
			// Keywords
			"break":       CategoryKeyword,
			"case":        CategoryKeyword,
			"chan":        CategoryKeyword,
			"const":       CategoryKeyword,
			"continue":    CategoryKeyword,
			"default":     CategoryKeyword,
			"defer":       CategoryKeyword,
			"else":        CategoryKeyword,
			"fallthrough": CategoryKeyword,
			"for":         CategoryKeyword,
			"func":        CategoryKeyword,
			"go":          CategoryKeyword,
			"goto":        CategoryKeyword,
			"if":          CategoryKeyword,
			"import":      CategoryKeyword,
			"interface":   CategoryKeyword,
			"map":         CategoryKeyword,
			"package":     CategoryKeyword,
			"range":       CategoryKeyword,
			"return":      CategoryKeyword,
			"select":      CategoryKeyword,
			"struct":      CategoryKeyword,
			"switch":      CategoryKeyword,
			"type":        CategoryKeyword,
			"var":         CategoryKeyword,

			// Literals
			"interpreted_string_literal": CategoryString,
			"raw_string_literal":         CategoryString,
			"rune_literal":               CategoryString,
			"int_literal":                CategoryNumber,
			"float_literal":              CategoryNumber,
			"imaginary_literal":          CategoryNumber,
			"true":                       CategoryBoolean,
			"false":                      CategoryBoolean,
			"nil":                        CategoryNil,

			// Comments
			"comment": CategoryComment,

			// Types - type_identifier is the primary type name node
			"type_identifier": CategoryType,

			// Built-in types (these appear as type_identifier in most contexts)
			// but sometimes as identifiers, so we rely on context rules

			// Operators
			"+":   CategoryOperator,
			"-":   CategoryOperator,
			"*":   CategoryOperator,
			"/":   CategoryOperator,
			"%":   CategoryOperator,
			"&":   CategoryOperator,
			"|":   CategoryOperator,
			"^":   CategoryOperator,
			"<<":  CategoryOperator,
			">>":  CategoryOperator,
			"&^":  CategoryOperator,
			"+=":  CategoryOperator,
			"-=":  CategoryOperator,
			"*=":  CategoryOperator,
			"/=":  CategoryOperator,
			"%=":  CategoryOperator,
			"&=":  CategoryOperator,
			"|=":  CategoryOperator,
			"^=":  CategoryOperator,
			"<<=": CategoryOperator,
			">>=": CategoryOperator,
			"&^=": CategoryOperator,
			"&&":  CategoryOperator,
			"||":  CategoryOperator,
			"<-":  CategoryOperator,
			"++":  CategoryOperator,
			"--":  CategoryOperator,
			"==":  CategoryOperator,
			"<":   CategoryOperator,
			">":   CategoryOperator,
			"=":   CategoryOperator,
			"!":   CategoryOperator,
			"!=":  CategoryOperator,
			"<=":  CategoryOperator,
			">=":  CategoryOperator,
			":=":  CategoryOperator,
			"...": CategoryOperator,

			// Punctuation
			"(": CategoryPunctuation,
			")": CategoryPunctuation,
			"[": CategoryPunctuation,
			"]": CategoryPunctuation,
			"{": CategoryPunctuation,
			"}": CategoryPunctuation,
			",": CategoryPunctuation,
			".": CategoryPunctuation,
			";": CategoryPunctuation,
			":": CategoryPunctuation,
		},
		ContextRules: []ContextRule{
			// Function definitions
			{
				NodeType:   "identifier",
				ParentType: "function_declaration",
				Field:      "name",
				Category:   CategoryFunction,
			},
			// Method definitions (receiver.method)
			{
				NodeType:   "field_identifier",
				ParentType: "method_declaration",
				Field:      "name",
				Category:   CategoryFunction,
			},
			// Function calls
			{
				NodeType:   "identifier",
				ParentType: "call_expression",
				Field:      "function",
				Category:   CategoryFunctionCall,
			},
			// Method calls (x.Method())
			{
				NodeType:   "field_identifier",
				ParentType: "selector_expression",
				Field:      "field",
				Category:   CategoryField, // Default to field; call_expression parent overrides
			},
			// Package name in package clause
			{
				NodeType:   "package_identifier",
				ParentType: "package_clause",
				Category:   CategoryNamespace,
			},
			// Import paths
			{
				NodeType:   "interpreted_string_literal",
				ParentType: "import_spec",
				Field:      "path",
				Category:   CategoryString,
			},
			// Struct field names in definitions
			{
				NodeType:   "field_identifier",
				ParentType: "field_declaration",
				Field:      "name",
				Category:   CategoryField,
			},
			// Struct tags
			{
				NodeType:   "raw_string_literal",
				ParentType: "field_declaration",
				Field:      "tag",
				Category:   CategoryTag,
			},
			// Parameter names
			{
				NodeType:   "identifier",
				ParentType: "parameter_declaration",
				Category:   CategoryParameter,
			},
			// Short var declaration names (x := ...)
			{
				NodeType:   "identifier",
				ParentType: "short_var_declaration",
				Field:      "left",
				Category:   CategoryVariable,
			},
			// Var declaration names
			{
				NodeType:   "identifier",
				ParentType: "var_spec",
				Field:      "name",
				Category:   CategoryVariable,
			},
			// Const declaration names
			{
				NodeType:   "identifier",
				ParentType: "const_spec",
				Field:      "name",
				Category:   CategoryConstant,
			},
			// Type declaration names
			{
				NodeType:   "type_identifier",
				ParentType: "type_spec",
				Field:      "name",
				Category:   CategoryType,
			},
		},
	}
}
