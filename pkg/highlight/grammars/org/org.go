// Package org provides tree-sitter bindings for the org-mode grammar.
// Source: https://github.com/milisims/tree-sitter-org
package org

/*
#cgo CFLAGS: -std=c11 -fPIC
#include "parser.inc"
#include "scanner.inc"
*/
import "C"
import "unsafe"

// GetLanguage returns the tree-sitter language for org.
func GetLanguage() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_org())
}
