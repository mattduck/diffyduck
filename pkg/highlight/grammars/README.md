# Vendored Tree-Sitter Grammars

This directory contains tree-sitter grammars that don't have official Go module
bindings. We vendor the C source code directly rather than depending on
third-party Go wrappers.

## Quick Start

Most grammars are committed directly, but SQL's parser is too large (~38MB).
Before building, download it with:

```bash
go generate ./pkg/highlight/grammars/...
```

## Why Vendor?

Tree-sitter grammars are written in JavaScript (grammar.js) which generates C
code (parser.c, and optionally scanner.c for external scanners). To use them
from Go, you need either:

1. **Official Go module**: Some grammars have official Go bindings published by
   tree-sitter (e.g., `github.com/tree-sitter/tree-sitter-go`). We use these
   when available.

2. **Vendored C code**: For grammars without official Go bindings, we copy the
   generated C code here. This avoids depending on third-party Go wrappers which
   could contain modified or malicious code.

## Directory Structure

Each vendored grammar has its own subdirectory:

```
grammars/
├── fetch_parsers.sh   # Downloads SQL parser (run via go generate)
├── README.md
├── diff/
│   ├── diff.go        # Go wrapper with CGO directives
│   └── parser.inc     # Generated parser (committed)
├── sql/
│   ├── sql.go         # Go wrapper with CGO directives
│   ├── .gitignore     # Parser files are gitignored (too large)
│   ├── parser.inc     # Generated parser (downloaded via go generate)
│   ├── scanner.inc    # External scanner (downloaded via go generate)
│   └── tree_sitter/
│       └── parser.h   # Header file (downloaded via go generate)
```

## Why .inc Instead of .c?

The C files are renamed from `.c` to `.inc` to prevent CGO from automatically
compiling them as separate compilation units. Instead, they're included via
`#include` directives in the Go wrapper file. This avoids duplicate symbol
errors when linking.

## Adding a New Vendored Grammar

1. Find the grammar repository (e.g., `github.com/user/tree-sitter-lang`)
2. Download `src/parser.c` and `src/scanner.c` (if it exists)
3. Download `src/tree_sitter/parser.h`
4. Rename `.c` files to `.inc`
5. Create a Go wrapper file (see existing examples)
6. Add the highlight query to `../queries/<lang>-vendored.scm`
7. Create `<lang>_lang.go` in the parent directory
8. Register in `registry.go`
9. If parser is large (>1MB), add to `.gitignore` and `fetch_parsers.sh`

## Updating a Grammar

To update a vendored grammar:

1. Download new `parser.c`/`scanner.c` from the upstream repository
2. Rename to `.inc`
3. Test that highlighting still works
4. Update the source URL comment in the Go wrapper file

## Current Vendored Grammars

| Language   | Source Repository                              | Committed? |
|------------|------------------------------------------------|------------|
| diff       | https://github.com/the-mikedavis/tree-sitter-diff | Yes |
| dockerfile | https://github.com/camdencheek/tree-sitter-dockerfile | Yes |
| elisp      | https://github.com/Wilfred/tree-sitter-elisp   | Yes |
| graphql    | https://github.com/bkegley/tree-sitter-graphql | Yes |
| ini        | https://github.com/justinmk/tree-sitter-ini    | Yes |
| markdown   | https://github.com/tree-sitter-grammars/tree-sitter-markdown | Yes |
| org        | https://github.com/milisims/tree-sitter-org    | Yes |
| sql        | https://github.com/DerekStride/tree-sitter-sql | No (38MB, use go generate) |
