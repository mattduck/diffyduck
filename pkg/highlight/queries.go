package highlight

import _ "embed"

// Highlight queries for tree-sitter syntax highlighting.
//
// # Architecture Overview
//
// We use tree-sitter for syntax highlighting, which requires two components:
//
//  1. Grammar (parser): Parses source code into a syntax tree
//  2. Query (highlights.scm): Defines which syntax nodes get which colors
//
// # Grammar Sources
//
// Grammars come from two sources:
//
//   - Go modules: Official tree-sitter bindings with Go wrappers (e.g.,
//     github.com/tree-sitter/tree-sitter-go). These are listed in go.mod.
//
//   - Vendored C code: For languages without official Go bindings, we vendor
//     the C source (parser.c, scanner.c) in pkg/highlight/grammars/. The .c
//     files are renamed to .inc to prevent CGO from auto-compiling them twice.
//
// # Query Sources
//
// All queries come from upstream tree-sitter grammar repositories, NOT from
// nvim-treesitter (which uses Neovim-specific predicates like #lua-match?).
//
// Naming convention:
//   - <lang>-v<version>.scm: Query for language with Go module (version from go.mod)
//   - <lang>-vendored.scm: Query for language with vendored C code
//
// # Query Precedence
//
// IMPORTANT: Upstream queries use "first match wins" semantics, but our
// highlighter uses "last match wins" (see MergeSpans in spans.go). This means
// queries may need reordering: general patterns (like @variable) should come
// BEFORE specific patterns (like @function).
//
// # Adding a New Language
//
//  1. If Go module exists: Add to go.mod, create <lang>_lang.go using the module
//  2. If no Go module: Vendor C code in pkg/highlight/grammars/<lang>/
//  3. Add query: Run ./queries/fetch_queries.sh or manually download
//  4. Reorder query if needed (general patterns first for "last wins")
//  5. Mark changes with "LOCAL MODIFICATION" comments
//  6. Register in registry.go and add to NewRegistry()
//  7. Test with sample files in highlight_samples/

// Languages with Go module bindings (version from go.mod)
//
//go:embed queries/bash-v0.25.1.scm
var bashHighlightQuery string

//go:embed queries/c-v0.24.1.scm
var cHighlightQuery string

//go:embed queries/css-v0.25.0.scm
var cssHighlightQuery string

//go:embed queries/go-v0.25.0.scm
var goHighlightQuery string

//go:embed queries/html-v0.23.2.scm
var htmlHighlightQuery string

//go:embed queries/javascript-v0.25.0.scm
var javascriptHighlightQuery string

//go:embed queries/json-v0.24.8.scm
var jsonHighlightQuery string

//go:embed queries/make-v1.1.1.scm
var makeHighlightQuery string

//go:embed queries/php-v0.24.2.scm
var phpHighlightQuery string

//go:embed queries/python-v0.25.0.scm
var pythonHighlightQuery string

//go:embed queries/rust-v0.24.0.scm
var rustHighlightQuery string

//go:embed queries/toml-v0.7.0.scm
var tomlHighlightQuery string

//go:embed queries/typescript-v0.23.2.scm
var typescriptHighlightQuery string

//go:embed queries/xml-v0.7.0.scm
var xmlHighlightQuery string

//go:embed queries/yaml-v0.7.2.scm
var yamlHighlightQuery string

// Languages with vendored C code (in pkg/highlight/grammars/)
//
//go:embed queries/diff-vendored.scm
var diffHighlightQuery string

//go:embed queries/dockerfile-vendored.scm
var dockerfileHighlightQuery string

//go:embed queries/elisp-vendored.scm
var elispHighlightQuery string

//go:embed queries/graphql-vendored.scm
var graphqlHighlightQuery string

//go:embed queries/ini-vendored.scm
var iniHighlightQuery string

//go:embed queries/markdown-vendored.scm
var markdownHighlightQuery string

//go:embed queries/org-vendored.scm
var orgHighlightQuery string

//go:embed queries/sql-vendored.scm
var sqlHighlightQuery string
