// Package markdown provides tree-sitter bindings for the markdown grammar.
// Source: https://github.com/tree-sitter-grammars/tree-sitter-markdown
package markdown

/*
#cgo CFLAGS: -std=c11 -fPIC
#include "parser.inc"
#include "scanner.inc"
*/
import "C"
import "unsafe"

// GetLanguage returns the tree-sitter language for markdown.
func GetLanguage() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_markdown())
}
