#!/bin/bash
#
# Fetches highlight queries from upstream tree-sitter grammar repositories.
#
# Usage: ./fetch_queries.sh
#
# This script downloads highlights.scm files from the official tree-sitter
# grammar repositories (NOT nvim-treesitter, which adds Lua-specific predicates
# like #lua-match? that don't work with the standard tree-sitter library).
#
# The upstream queries use "first match wins" semantics, but our highlighter
# uses "last match wins" (see MergeSpans in spans.go). After fetching, queries
# may need manual reordering: general patterns first, specific patterns later.
#
# WARNING: This will overwrite any local modifications to the query files!
# Some queries have local modifications marked with "LOCAL MODIFICATION" comments.
# After running this script, check `git diff` and re-apply those changes.
#
# After fetching, run tests to verify compatibility:
#   go test ./pkg/highlight/...

set -e

cd "$(dirname "$0")"

# Language configurations
# Format: "output-name:go-mod-pattern:github-repo"
# Note: Some grammars are under tree-sitter/, others under tree-sitter-grammars/
LANGUAGES=(
    "go:tree-sitter-go:tree-sitter/tree-sitter-go"
    "python:tree-sitter-python:tree-sitter/tree-sitter-python"
    "yaml:tree-sitter-yaml:tree-sitter-grammars/tree-sitter-yaml"
    "toml:tree-sitter-toml:tree-sitter-grammars/tree-sitter-toml"
)

get_version() {
    local pattern="$1"
    local gomod="../../../go.mod"
    grep "${pattern}" "${gomod}" | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | head -1
}

echo "Fetching queries from upstream tree-sitter grammar repositories"
echo ""

for entry in "${LANGUAGES[@]}"; do
    IFS=':' read -r name pattern repo <<< "${entry}"

    version=$(get_version "${pattern}")
    if [ -z "${version}" ]; then
        echo "WARNING: Could not find version for ${pattern} in go.mod, skipping ${name}"
        continue
    fi

    # Try to fetch from the tag matching our version, fall back to master
    url="https://raw.githubusercontent.com/${repo}/refs/tags/${version}/queries/highlights.scm"
    fallback_url="https://raw.githubusercontent.com/${repo}/master/queries/highlights.scm"
    output="${name}-${version}.scm"

    echo "Fetching ${name} (${version}) from ${repo}..."

    tmpfile=$(mktemp)
    cat > "${tmpfile}" << EOF
; Source: github.com/${repo} @ ${version}
; Upstream: https://github.com/${repo}/blob/${version}/queries/highlights.scm
;
; IMPORTANT: Upstream queries use "first match wins" but our highlighter uses
; "last match wins" (see MergeSpans). Queries may need reordering after fetching:
; general patterns (like @variable) should come BEFORE specific patterns (like @function).
;
; Check for LOCAL MODIFICATION comments below for any manual changes.

EOF

    if curl -sfL "${url}" >> "${tmpfile}" 2>/dev/null; then
        mv "${tmpfile}" "${output}"
        echo "  OK (from tag ${version})"
    elif curl -sfL "${fallback_url}" >> "${tmpfile}" 2>/dev/null; then
        # Update header to note we used master
        sed -i.bak "s/@ ${version}/@ master (tag ${version} not found)/" "${tmpfile}"
        rm -f "${tmpfile}.bak"
        mv "${tmpfile}" "${output}"
        echo "  OK (from master, tag ${version} not found)"
    else
        rm -f "${tmpfile}"
        echo "  FAILED"
    fi
done

echo ""
echo "Done. Next steps:"
echo "  1. Check 'git diff' for changes to query files"
echo "  2. Reorder patterns if needed (general before specific for 'last wins')"
echo "  3. Re-apply any LOCAL MODIFICATION sections (grep for 'LOCAL MODIFICATION')"
echo "  4. Run 'go test ./pkg/highlight/...' to verify compatibility"
