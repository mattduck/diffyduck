// Package graphql provides tree-sitter bindings for the graphql grammar.
// Source: https://github.com/bkegley/tree-sitter-graphql
package graphql

/*
#cgo CFLAGS: -std=c11 -fPIC
#include "parser.inc"
*/
import "C"
import "unsafe"

// GetLanguage returns the tree-sitter language for graphql.
func GetLanguage() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_graphql())
}
