// Package ini provides tree-sitter bindings for the ini grammar.
// Source: https://github.com/justinmk/tree-sitter-ini
package ini

/*
#cgo CFLAGS: -std=c11 -fPIC
#include "parser.inc"
*/
import "C"
import "unsafe"

// GetLanguage returns the tree-sitter language for ini.
func GetLanguage() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_ini())
}
