#!/bin/bash
#
# Downloads the SQL tree-sitter parser files.
#
# The SQL parser is ~38MB so we don't commit it. Run this before building:
#
#   ./fetch_parsers.sh
#   go generate ./pkg/highlight/grammars/...
#
# Other vendored grammars are small enough to commit directly.

set -e

cd "$(dirname "$0")"

echo "Fetching SQL parser from DerekStride/tree-sitter-sql (gh-pages)..."

mkdir -p sql/tree_sitter

# Download parser.c
echo "  Downloading parser.inc (this may take a moment, file is ~38MB)..."
if curl -sfL "https://raw.githubusercontent.com/DerekStride/tree-sitter-sql/gh-pages/src/parser.c" -o sql/parser.inc; then
    echo "  ✓ parser.inc"
else
    echo "  ✗ Failed to download parser.c"
    exit 1
fi

# Download scanner.c
if curl -sfL "https://raw.githubusercontent.com/DerekStride/tree-sitter-sql/gh-pages/src/scanner.c" -o sql/scanner.inc; then
    echo "  ✓ scanner.inc"
else
    echo "  ✗ Failed to download scanner.c"
    exit 1
fi

# Download tree_sitter/parser.h
if curl -sfL "https://raw.githubusercontent.com/DerekStride/tree-sitter-sql/gh-pages/src/tree_sitter/parser.h" -o sql/tree_sitter/parser.h; then
    echo "  ✓ tree_sitter/parser.h"
else
    echo "  ✗ Failed to download tree_sitter/parser.h"
    exit 1
fi

echo ""
echo "SQL parser downloaded successfully."
