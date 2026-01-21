package structure

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// goExtractor extracts structure from Go source code.
type goExtractor struct{}

// Go structural node types and their display kind prefixes.
var goStructuralTypes = map[string]string{
	"function_declaration": "func",
	"method_declaration":   "func",
	"type_declaration":     "type",
}

// Extract implements languageExtractor for Go.
func (g *goExtractor) Extract(tree *tree_sitter.Tree, content []byte) []Entry {
	if tree == nil {
		return nil
	}

	var entries []Entry
	g.walkNode(tree.RootNode(), content, &entries)
	return entries
}

// walkNode recursively walks the tree and collects structural entries.
func (g *goExtractor) walkNode(node *tree_sitter.Node, content []byte, entries *[]Entry) {
	if node == nil {
		return
	}

	nodeType := node.Kind()

	// Check if this is a structural node type
	if kind, ok := goStructuralTypes[nodeType]; ok {
		name := g.extractName(node, nodeType, content)
		if name != "" {
			// tree-sitter positions are 0-based, we want 1-based
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
		g.walkNode(child, content, entries)
	}
}

// extractName extracts the name from a structural node.
func (g *goExtractor) extractName(node *tree_sitter.Node, nodeType string, content []byte) string {
	switch nodeType {
	case "function_declaration", "method_declaration":
		// The "name" field contains the function/method name
		nameNode := node.ChildByFieldName("name")
		if nameNode != nil {
			return nameNode.Utf8Text(content)
		}

	case "type_declaration":
		// type_declaration contains one or more type_spec children
		// Each type_spec has a "name" field
		childCount := node.ChildCount()
		for i := uint(0); i < uint(childCount); i++ {
			child := node.Child(i)
			if child != nil && child.Kind() == "type_spec" {
				nameNode := child.ChildByFieldName("name")
				if nameNode != nil {
					return nameNode.Utf8Text(content)
				}
			}
		}
	}

	return ""
}
