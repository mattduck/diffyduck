package structure

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// pythonExtractor extracts structure from Python source code.
type pythonExtractor struct{}

// Python structural node types and their display kind prefixes.
var pythonStructuralTypes = map[string]string{
	"function_definition": "def",
	"class_definition":    "class",
}

// Extract implements languageExtractor for Python.
func (p *pythonExtractor) Extract(tree *tree_sitter.Tree, content []byte) []Entry {
	if tree == nil {
		return nil
	}

	var entries []Entry
	p.walkNode(tree.RootNode(), content, &entries)
	return entries
}

// walkNode recursively walks the tree and collects structural entries.
func (p *pythonExtractor) walkNode(node *tree_sitter.Node, content []byte, entries *[]Entry) {
	if node == nil {
		return
	}

	nodeType := node.Kind()

	// Handle decorated definitions (functions/classes with @decorators)
	if nodeType == "decorated_definition" {
		// The actual function/class is in the "definition" field
		defNode := node.ChildByFieldName("definition")
		if defNode != nil {
			defType := defNode.Kind()
			if kind, ok := pythonStructuralTypes[defType]; ok {
				entry := p.extractEntry(defNode, kind, content)
				if entry != nil {
					// Use the decorated_definition's span (includes decorators)
					entry.StartLine = int(node.StartPosition().Row) + 1
					entry.EndLine = int(node.EndPosition().Row) + 1
					*entries = append(*entries, *entry)
				}
			}
			// Recurse into the definition to find nested structures (e.g., methods in decorated classes)
			p.walkNode(defNode, content, entries)
		}
		// Don't recurse into other children (decorators) to avoid issues
		return
	}

	// Check if this is a structural node type
	if kind, ok := pythonStructuralTypes[nodeType]; ok {
		// Skip if parent is decorated_definition (already handled above)
		parent := node.Parent()
		if parent != nil && parent.Kind() == "decorated_definition" {
			// Still recurse into children (e.g., methods in a decorated class)
			childCount := node.ChildCount()
			for i := uint(0); i < uint(childCount); i++ {
				child := node.Child(i)
				p.walkNode(child, content, entries)
			}
			return
		}

		entry := p.extractEntry(node, kind, content)
		if entry != nil {
			*entries = append(*entries, *entry)
		}
	}

	// Recurse into children
	childCount := node.ChildCount()
	for i := uint(0); i < uint(childCount); i++ {
		child := node.Child(i)
		p.walkNode(child, content, entries)
	}
}

// extractEntry extracts an Entry from a structural node.
func (p *pythonExtractor) extractEntry(node *tree_sitter.Node, kind string, content []byte) *Entry {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Utf8Text(content)

	entry := &Entry{
		StartLine: int(node.StartPosition().Row) + 1,
		EndLine:   int(node.EndPosition().Row) + 1,
		Name:      name,
		Kind:      kind,
	}

	// Only functions have parameters and return types
	if node.Kind() == "function_definition" {
		entry.Params = p.extractParams(node, content)
		entry.ReturnType = p.extractReturnType(node, content)
	}

	return entry
}

// extractReturnType extracts the return type annotation from a function definition.
// Returns e.g., "None" or "" if no return type is specified.
func (p *pythonExtractor) extractReturnType(node *tree_sitter.Node, content []byte) string {
	returnTypeNode := node.ChildByFieldName("return_type")
	if returnTypeNode == nil {
		return ""
	}
	return normalizeWhitespace(returnTypeNode.Utf8Text(content))
}

// extractParams extracts the parameters from a function definition.
// Walks the AST to extract only parameter nodes, filtering out comments.
func (p *pythonExtractor) extractParams(node *tree_sitter.Node, content []byte) []string {
	paramsNode := node.ChildByFieldName("parameters")
	if paramsNode == nil {
		return nil
	}

	var params []string
	childCount := paramsNode.ChildCount()
	for i := uint(0); i < uint(childCount); i++ {
		child := paramsNode.Child(i)
		kind := child.Kind()

		// Skip punctuation and comments
		if kind == "(" || kind == ")" || kind == "," || kind == "comment" {
			continue
		}

		// Extract the parameter text, normalized for multiline cases
		params = append(params, normalizeWhitespace(child.Utf8Text(content)))
	}

	return params
}
