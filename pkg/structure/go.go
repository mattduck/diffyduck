package structure

import (
	"strings"

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
		entry := g.extractEntry(node, nodeType, kind, content)
		if entry != nil {
			*entries = append(*entries, *entry)
		}
	}

	// Recurse into children
	childCount := node.ChildCount()
	for i := uint(0); i < uint(childCount); i++ {
		child := node.Child(i)
		g.walkNode(child, content, entries)
	}
}

// extractEntry extracts an Entry from a structural node.
func (g *goExtractor) extractEntry(node *tree_sitter.Node, nodeType, kind string, content []byte) *Entry {
	switch nodeType {
	case "function_declaration":
		nameNode := node.ChildByFieldName("name")
		if nameNode == nil {
			return nil
		}
		return &Entry{
			StartLine:  int(node.StartPosition().Row) + 1,
			EndLine:    int(node.EndPosition().Row) + 1,
			Name:       nameNode.Utf8Text(content),
			Kind:       kind,
			Params:     g.extractParams(node, content),
			ReturnType: g.extractReturnType(node, content),
		}

	case "method_declaration":
		nameNode := node.ChildByFieldName("name")
		if nameNode == nil {
			return nil
		}
		return &Entry{
			StartLine:  int(node.StartPosition().Row) + 1,
			EndLine:    int(node.EndPosition().Row) + 1,
			Name:       nameNode.Utf8Text(content),
			Kind:       kind,
			Receiver:   g.extractReceiver(node, content),
			Params:     g.extractParams(node, content),
			ReturnType: g.extractReturnType(node, content),
		}

	case "type_declaration":
		// type_declaration contains one or more type_spec children
		childCount := node.ChildCount()
		for i := uint(0); i < uint(childCount); i++ {
			child := node.Child(i)
			if child != nil && child.Kind() == "type_spec" {
				nameNode := child.ChildByFieldName("name")
				if nameNode != nil {
					return &Entry{
						StartLine: int(node.StartPosition().Row) + 1,
						EndLine:   int(node.EndPosition().Row) + 1,
						Name:      nameNode.Utf8Text(content),
						Kind:      kind,
					}
				}
			}
		}
	}

	return nil
}

// extractReceiver extracts the receiver from a method declaration.
// Returns e.g., "(m Model)" or "(m *Model)".
func (g *goExtractor) extractReceiver(node *tree_sitter.Node, content []byte) string {
	receiverNode := node.ChildByFieldName("receiver")
	if receiverNode == nil {
		return ""
	}
	// Receiver is a parameter_list with typically one parameter
	params := g.extractParamSlice(receiverNode, content)
	if len(params) == 0 {
		return ""
	}
	return "(" + params[0] + ")"
}

// extractParams extracts the parameters from a function/method declaration.
func (g *goExtractor) extractParams(node *tree_sitter.Node, content []byte) []string {
	paramsNode := node.ChildByFieldName("parameters")
	if paramsNode == nil {
		return nil
	}
	return g.extractParamSlice(paramsNode, content)
}

// extractReturnType extracts the return type from a function/method declaration.
// Returns e.g., "error", "(string, error)", "*User", or "" if no return type.
func (g *goExtractor) extractReturnType(node *tree_sitter.Node, content []byte) string {
	resultNode := node.ChildByFieldName("result")
	if resultNode == nil {
		return ""
	}
	return normalizeWhitespace(resultNode.Utf8Text(content))
}

// extractParamSlice extracts parameters from a parameter_list node as a slice.
func (g *goExtractor) extractParamSlice(node *tree_sitter.Node, content []byte) []string {
	var params []string
	childCount := node.ChildCount()
	for i := uint(0); i < uint(childCount); i++ {
		child := node.Child(i)
		kind := child.Kind()

		// Skip punctuation
		if kind == "(" || kind == ")" || kind == "," {
			continue
		}

		// Extract parameter text, normalized
		paramText := normalizeWhitespace(child.Utf8Text(content))
		// Clean trailing commas in nested structures like generics: [K, V,] -> [K, V]
		paramText = strings.ReplaceAll(paramText, ",]", "]")
		params = append(params, paramText)
	}

	return params
}
