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
		name, signature := g.extractNameAndSignature(node, nodeType, content)
		if name != "" {
			// tree-sitter positions are 0-based, we want 1-based
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
		g.walkNode(child, content, entries)
	}
}

// extractNameAndSignature extracts the name and signature from a structural node.
// Returns (name, signature) where signature includes receiver and params for functions.
func (g *goExtractor) extractNameAndSignature(node *tree_sitter.Node, nodeType string, content []byte) (string, string) {
	switch nodeType {
	case "function_declaration":
		nameNode := node.ChildByFieldName("name")
		if nameNode == nil {
			return "", ""
		}
		name := nameNode.Utf8Text(content)
		params := g.extractParams(node, content)
		signature := name + params
		return name, signature

	case "method_declaration":
		nameNode := node.ChildByFieldName("name")
		if nameNode == nil {
			return "", ""
		}
		name := nameNode.Utf8Text(content)
		receiver := g.extractReceiver(node, content)
		params := g.extractParams(node, content)
		signature := receiver + name + params
		return name, signature

	case "type_declaration":
		// type_declaration contains one or more type_spec children
		childCount := node.ChildCount()
		for i := uint(0); i < uint(childCount); i++ {
			child := node.Child(i)
			if child != nil && child.Kind() == "type_spec" {
				nameNode := child.ChildByFieldName("name")
				if nameNode != nil {
					return nameNode.Utf8Text(content), ""
				}
			}
		}
	}

	return "", ""
}

// extractReceiver extracts the receiver from a method declaration.
// Returns e.g., "(m Model) " or "(m *Model) "
// Normalizes multiline receivers to a single line.
func (g *goExtractor) extractReceiver(node *tree_sitter.Node, content []byte) string {
	receiverNode := node.ChildByFieldName("receiver")
	if receiverNode == nil {
		return ""
	}
	// Get the full receiver text including parens
	return normalizeWhitespace(receiverNode.Utf8Text(content)) + " "
}

// extractParams extracts the parameters from a function/method declaration.
// Returns e.g., "(ctx, name string)" or "()"
// Normalizes multiline parameters to a single line.
func (g *goExtractor) extractParams(node *tree_sitter.Node, content []byte) string {
	paramsNode := node.ChildByFieldName("parameters")
	if paramsNode == nil {
		return "()"
	}
	return normalizeWhitespace(paramsNode.Utf8Text(content))
}
