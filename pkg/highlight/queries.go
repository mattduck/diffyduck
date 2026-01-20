package highlight

import _ "embed"

// Highlight queries for tree-sitter syntax highlighting.
//
// IMPORTANT: Query source and precedence semantics
//
// We use queries from upstream tree-sitter grammar repositories:
//   - github.com/tree-sitter/tree-sitter-go
//   - github.com/tree-sitter/tree-sitter-python
//   - github.com/tree-sitter-grammars/tree-sitter-yaml
//   - github.com/tree-sitter-grammars/tree-sitter-toml
//
// We do NOT use nvim-treesitter queries because they include Neovim-specific
// predicates like #lua-match? that don't work with the standard tree-sitter library.
//
// Upstream queries use "first match wins" semantics, but our highlighter uses
// "last match wins" (see MergeSpans in spans.go). This means queries may need
// reordering: general patterns first, specific patterns later:
//
//   (identifier) @variable           ; general - matches all identifiers
//   (function_declaration
//     name: (identifier) @function)  ; specific - overrides @variable for func names
//
// When adding a new language:
//
// 1. Run ./queries/fetch_queries.sh to fetch from upstream
// 2. Reorder if needed: general patterns first, specific last
// 3. Mark any changes with "LOCAL MODIFICATION" comments
// 4. Test that specific constructs (functions, types, etc.) aren't incorrectly
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
