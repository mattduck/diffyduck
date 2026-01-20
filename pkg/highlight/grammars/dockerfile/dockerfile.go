// Package dockerfile provides tree-sitter bindings for the dockerfile grammar.
// Source: https://github.com/camdencheek/tree-sitter-dockerfile
package dockerfile

/*
#cgo CFLAGS: -std=c11 -fPIC
#include "parser.inc"
#include "scanner.inc"
*/
import "C"
import "unsafe"

// GetLanguage returns the tree-sitter language for dockerfile.
func GetLanguage() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_dockerfile())
}
