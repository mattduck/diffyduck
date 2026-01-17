// Package highlight provides syntax highlighting using tree-sitter.
package highlight

// Category represents a semantic category for syntax highlighting.
// All languages map their tree-sitter node types to these universal categories.
type Category int

const (
	CategoryNone Category = iota

	// Basics
	CategoryKeyword     // func, if, else, return, import, etc.
	CategoryOperator    // +, -, *, /, =, ==, etc.
	CategoryPunctuation // (), {}, [], ;, etc.

	// Literals
	CategoryString  // "hello", 'c', `raw`
	CategoryNumber  // 42, 3.14, 0xFF
	CategoryBoolean // true, false
	CategoryNil     // nil, null, None

	// Names
	CategoryFunction     // function/method names at definition
	CategoryFunctionCall // function/method names at call site
	CategoryType         // type names
	CategoryVariable     // variable names
	CategoryConstant     // constant names (ALL_CAPS convention)
	CategoryParameter    // function parameters
	CategoryField        // struct/object fields

	// Comments
	CategoryComment    // // or /* */
	CategoryDocComment // /// or /** */ (if distinguishable)

	// Special
	CategoryTag       // struct tags, annotations
	CategoryAttribute // @decorators, #[attributes]
	CategoryNamespace // package names, module names
)

// String returns the name of the category.
func (c Category) String() string {
	switch c {
	case CategoryNone:
		return "None"
	case CategoryKeyword:
		return "Keyword"
	case CategoryOperator:
		return "Operator"
	case CategoryPunctuation:
		return "Punctuation"
	case CategoryString:
		return "String"
	case CategoryNumber:
		return "Number"
	case CategoryBoolean:
		return "Boolean"
	case CategoryNil:
		return "Nil"
	case CategoryFunction:
		return "Function"
	case CategoryFunctionCall:
		return "FunctionCall"
	case CategoryType:
		return "Type"
	case CategoryVariable:
		return "Variable"
	case CategoryConstant:
		return "Constant"
	case CategoryParameter:
		return "Parameter"
	case CategoryField:
		return "Field"
	case CategoryComment:
		return "Comment"
	case CategoryDocComment:
		return "DocComment"
	case CategoryTag:
		return "Tag"
	case CategoryAttribute:
		return "Attribute"
	case CategoryNamespace:
		return "Namespace"
	default:
		return "Unknown"
	}
}
