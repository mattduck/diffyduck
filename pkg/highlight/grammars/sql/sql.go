// Package sql provides tree-sitter bindings for the SQL grammar.
// Source: https://github.com/DerekStride/tree-sitter-sql
//
// The parser files (parser.inc, scanner.inc) are not committed due to their
// size (~38MB). Run go generate to download them before building:
//
//	go generate ./pkg/highlight/grammars/...
//
//go:generate ../fetch_parsers.sh
package sql

/*
#cgo CFLAGS: -std=c11 -fPIC
#include "parser.inc"
#include "scanner.inc"
*/
import "C"
import "unsafe"

// GetLanguage returns the tree-sitter language for SQL.
func GetLanguage() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_sql())
}
