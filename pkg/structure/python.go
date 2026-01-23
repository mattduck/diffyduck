package structure

import (
	"strings"

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
				name, signature := p.extractNameAndSignature(defNode, content)
				if name != "" {
					// Use the decorated_definition's span (includes decorators)
					startPos := node.StartPosition()
					endPos := node.EndPosition()

					*entries = append(*entries, Entry{
						StartLine: int(startPos.Row) + 1,
						EndLine:   int(endPos.Row) + 1,
						Name:      name,
						Kind:      kind,
						Signature: signature,
					})
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

		name, signature := p.extractNameAndSignature(node, content)
		if name != "" {
			startPos := node.StartPosition()
			endPos := node.EndPosition()

			*entries = append(*entries, Entry{
				StartLine: int(startPos.Row) + 1,
				EndLine:   int(endPos.Row) + 1,
				Name:      name,
				Kind:      kind,
				Signature: signature,
			})
		}
	}

	// Recurse into children
	childCount := node.ChildCount()
	for i := uint(0); i < uint(childCount); i++ {
		child := node.Child(i)
		p.walkNode(child, content, entries)
	}
}

// extractNameAndSignature extracts the name and signature from a structural node.
func (p *pythonExtractor) extractNameAndSignature(node *tree_sitter.Node, content []byte) (string, string) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return "", ""
	}
	name := nameNode.Utf8Text(content)

	// Only functions have parameters
	if node.Kind() == "function_definition" {
		params := p.extractParams(node, content)
		return name, name + params
	}

	// Classes don't have a signature
	return name, ""
}

// extractParams extracts the parameters from a function definition.
// Walks the AST to extract only parameter nodes, filtering out comments
// and trailing commas for a clean signature.
func (p *pythonExtractor) extractParams(node *tree_sitter.Node, content []byte) string {
	paramsNode := node.ChildByFieldName("parameters")
	if paramsNode == nil {
		return "()"
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

	return "(" + strings.Join(params, ", ") + ")"
}
