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
				name := p.extractName(defNode, content)
				if name != "" {
					// Use the decorated_definition's span (includes decorators)
					startPos := node.StartPosition()
					endPos := node.EndPosition()

					*entries = append(*entries, Entry{
						StartLine: int(startPos.Row) + 1,
						EndLine:   int(endPos.Row) + 1,
						Name:      name,
						Kind:      kind,
					})
				}
			}
		}
		// Don't recurse into decorated_definition children to avoid double-counting
		return
	}

	// Check if this is a structural node type
	if kind, ok := pythonStructuralTypes[nodeType]; ok {
		name := p.extractName(node, content)
		if name != "" {
			startPos := node.StartPosition()
			endPos := node.EndPosition()

			*entries = append(*entries, Entry{
				StartLine: int(startPos.Row) + 1,
				EndLine:   int(endPos.Row) + 1,
				Name:      name,
				Kind:      kind,
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

// extractName extracts the name from a structural node.
func (p *pythonExtractor) extractName(node *tree_sitter.Node, content []byte) string {
	// Both function_definition and class_definition have a "name" field
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return nameNode.Utf8Text(content)
	}
	return ""
}
