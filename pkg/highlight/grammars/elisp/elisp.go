// Package elisp provides tree-sitter bindings for the emacs-lisp grammar.
// Source: https://github.com/Wilfred/tree-sitter-elisp
package elisp

/*
#cgo CFLAGS: -std=c11 -fPIC
#include "parser.inc"
*/
import "C"
import "unsafe"

// GetLanguage returns the tree-sitter language for elisp.
func GetLanguage() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_elisp())
}
