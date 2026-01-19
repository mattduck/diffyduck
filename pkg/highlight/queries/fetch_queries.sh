#!/bin/bash
#
# Fetches highlight queries from nvim-treesitter and writes them to versioned files.
#
# Usage: ./fetch_queries.sh [nvim-treesitter-tag]
#
# Example: ./fetch_queries.sh v0.9.2
#
# This script downloads highlights.scm files from the nvim-treesitter repository.
# The queries are designed for "last match wins" semantics which matches our
# highlighter implementation (see MergeSpans in spans.go).
#
# WARNING: This will overwrite any local modifications to the query files!
# Some queries have local modifications marked with "LOCAL MODIFICATION" comments.
# After running this script, check `git diff` and re-apply those changes.
#
# After fetching, run tests to verify compatibility:
#   go test ./pkg/highlight/...
#
# Note: nvim-treesitter queries may include directives we don't handle (like
# #set! priority or @spell). Tree-sitter ignores unknown predicates, so these
# are harmless but won't have any effect in our highlighter.

set -e

cd "$(dirname "$0")"

NVIM_TS_TAG="${1:-master}"
BASE_URL="https://raw.githubusercontent.com/nvim-treesitter/nvim-treesitter/${NVIM_TS_TAG}/queries"

# Map language -> grammar package -> version (extracted from go.mod)
# Format: "lang:package-pattern:output-name"
LANGUAGES=(
    "go:tree-sitter-go:go"
    "python:tree-sitter-python:python"
    "yaml:tree-sitter-yaml:yaml"
    "toml:tree-sitter-toml:toml"
)

get_version() {
    local pattern="$1"
    local gomod="../../../go.mod"
    # Look for the package in go.mod and extract version
    grep "${pattern}" "${gomod}" | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | head -1
}

echo "Fetching queries from nvim-treesitter @ ${NVIM_TS_TAG}"
echo ""

for entry in "${LANGUAGES[@]}"; do
    IFS=':' read -r lang pattern name <<< "${entry}"

    version=$(get_version "${pattern}")
    if [ -z "${version}" ]; then
        echo "WARNING: Could not find version for ${pattern} in go.mod, skipping ${lang}"
        continue
    fi

    url="${BASE_URL}/${lang}/highlights.scm"
    output="${name}-${version}.scm"

    echo "Fetching ${lang} -> ${output}"

    # Create temp file with header
    tmpfile=$(mktemp)
    cat > "${tmpfile}" << EOF
; Source: nvim-treesitter @ ${NVIM_TS_TAG}
; URL: ${url}
; Grammar: ${pattern} ${version}
;
; This query uses "last match wins" semantics - general patterns should come
; before specific patterns so that specific patterns override them.
; See queries.go for more details.

EOF

    if curl -sfL "${url}" >> "${tmpfile}"; then
        mv "${tmpfile}" "${output}"
        echo "  OK"
    else
        rm -f "${tmpfile}"
        echo "  FAILED (query may not exist for ${lang})"
    fi
done

echo ""
echo "Done. Next steps:"
echo "  1. Check 'git diff' for changes to query files"
echo "  2. Re-apply any LOCAL MODIFICATION sections (grep for 'LOCAL MODIFICATION')"
echo "  3. Run 'go test ./pkg/highlight/...' to verify compatibility"
