package highlight

import _ "embed"

// Highlight queries for tree-sitter syntax highlighting.
//
// IMPORTANT: Query source and precedence semantics
//
// There are two main sources for tree-sitter highlight queries:
//
// 1. Upstream grammar repos (e.g., github.com/tree-sitter/tree-sitter-go)
//    These queries are designed for the tree-sitter CLI which uses "first match wins"
//    semantics - earlier patterns in the file take precedence.
//
// 2. nvim-treesitter (github.com/nvim-treesitter/nvim-treesitter/queries/{lang}/)
//    These queries are forked from upstream but reordered for "last match wins"
//    semantics - later patterns override earlier ones. This is what Neovim uses.
//
// Our highlighter uses "last match wins" (see MergeSpans in spans.go), so queries
// must be ordered with general patterns first and specific patterns later:
//
//   (identifier) @variable           ; general - matches all identifiers
//   (function_declaration
//     name: (identifier) @function)  ; specific - overrides @variable for func names
//
// We currently use nvim-treesitter's queries (or manually reorder upstream queries)
// to match this expectation. When adding a new language:
//
// 1. Check nvim-treesitter first: queries/{lang}/highlights.scm
// 2. If not available, use upstream and reorder: general patterns first, specific last
// 3. Test that specific constructs (functions, types, etc.) aren't incorrectly
//    highlighted as generic variables/strings
//
// Version compatibility: queries may need updates when grammar versions change,
// as node types and tree structure can evolve. The filename includes the grammar
// version for tracking.

//go:embed queries/go-v0.25.0.scm
var goHighlightQuery string

//go:embed queries/python-v0.25.0.scm
var pythonHighlightQuery string

//go:embed queries/yaml-v0.7.2.scm
var yamlHighlightQuery string

//go:embed queries/toml-v0.7.0.scm
var tomlHighlightQuery string
