// Package diff provides tree-sitter bindings for the diff grammar.
// Source: https://github.com/the-mikedavis/tree-sitter-diff
package diff

/*
#cgo CFLAGS: -std=c11 -fPIC
#include "parser.inc"
*/
import "C"
import "unsafe"

// GetLanguage returns the tree-sitter language for diff.
func GetLanguage() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_diff())
}
