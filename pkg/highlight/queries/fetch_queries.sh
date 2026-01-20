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

# Don't exit on failure - we want to see all results
# set -e

cd "$(dirname "$0")"

# Languages with official Go bindings (version from go.mod)
# Format: "output-name:github-repo:query-path" (query-path optional, defaults to queries/highlights.scm)
# Version is extracted from go.mod using the full module path
# Output files: <name>-v<version>.scm
OFFICIAL_LANGUAGES=(
    "bash:tree-sitter/tree-sitter-bash:"
    "c:tree-sitter/tree-sitter-c:"
    "css:tree-sitter/tree-sitter-css:"
    "go:tree-sitter/tree-sitter-go:"
    "html:tree-sitter/tree-sitter-html:"
    "javascript:tree-sitter/tree-sitter-javascript:"
    "json:tree-sitter/tree-sitter-json:"
    "make:tree-sitter-grammars/tree-sitter-make:"
    "php:tree-sitter/tree-sitter-php:"
    "python:tree-sitter/tree-sitter-python:"
    "rust:tree-sitter/tree-sitter-rust:"
    "toml:tree-sitter-grammars/tree-sitter-toml:"
    "typescript:tree-sitter/tree-sitter-typescript:"
    "xml:tree-sitter-grammars/tree-sitter-xml:queries/xml/highlights.scm"
    "yaml:tree-sitter-grammars/tree-sitter-yaml:"
)

# Languages with vendored C code in pkg/highlight/grammars/ (fixed branch reference)
# Format: "output-name:branch:github-repo:query-path"
# query-path is optional, defaults to "queries/highlights.scm"
# Output files: <name>-vendored.scm
VENDORED_LANGUAGES=(
    "diff:main:the-mikedavis/tree-sitter-diff:"
    "dockerfile:main:camdencheek/tree-sitter-dockerfile:"
    "elisp:main:Wilfred/tree-sitter-elisp:"
    "graphql:master:bkegley/tree-sitter-graphql:queries/graphql/highlights.scm"
    "ini:master:justinmk/tree-sitter-ini:"
    "markdown:split_parser:tree-sitter-grammars/tree-sitter-markdown:tree-sitter-markdown/queries/highlights.scm"
    "org:main:milisims/tree-sitter-org:"
    "sql:gh-pages:DerekStride/tree-sitter-sql:"
)

get_version() {
    local repo="$1"
    local gomod="../../../go.mod"
    # Match the full module path (github.com/org/repo) to avoid partial matches
    grep "github.com/${repo} " "${gomod}" | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | head -1
}

fetch_query() {
    local name="$1"
    local version="$2"
    local repo="$3"
    local output="$4"

    local url="https://raw.githubusercontent.com/${repo}/refs/tags/${version}/queries/highlights.scm"
    local fallback_url="https://raw.githubusercontent.com/${repo}/${version}/queries/highlights.scm"

    echo "Fetching ${name} (${version}) from ${repo}..."

    local tmpfile=$(mktemp)
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
        return 0
    elif curl -sfL "${fallback_url}" >> "${tmpfile}" 2>/dev/null; then
        mv "${tmpfile}" "${output}"
        echo "  OK (from branch ${version})"
        return 0
    else
        rm -f "${tmpfile}"
        echo "  FAILED"
        return 1
    fi
}

echo "Fetching queries from upstream tree-sitter grammar repositories"
echo ""

echo "=== Official Go bindings (version from go.mod) ==="
for entry in "${OFFICIAL_LANGUAGES[@]}"; do
    IFS=':' read -r name repo query_path <<< "${entry}"

    version=$(get_version "${repo}")
    if [ -z "${version}" ]; then
        echo "WARNING: Could not find version for ${repo} in go.mod, skipping ${name}"
        continue
    fi

    # Default query path if not specified
    if [ -z "${query_path}" ]; then
        query_path="queries/highlights.scm"
    fi

    output="${name}-${version}.scm"

    echo "Fetching ${name} (${version}) from ${repo}..."

    # Try tag first, then branch
    url="https://raw.githubusercontent.com/${repo}/refs/tags/${version}/${query_path}"
    fallback_url="https://raw.githubusercontent.com/${repo}/${version}/${query_path}"

    tmpfile=$(mktemp)
    cat > "${tmpfile}" << EOF
; Source: github.com/${repo} @ ${version}
; Upstream: https://github.com/${repo}/blob/${version}/${query_path}
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
        mv "${tmpfile}" "${output}"
        echo "  OK (from branch ${version})"
    else
        rm -f "${tmpfile}"
        echo "  FAILED"
    fi
done

echo ""
echo "=== Vendored languages (fixed branch) ==="
for entry in "${VENDORED_LANGUAGES[@]}"; do
    IFS=':' read -r name branch repo query_path <<< "${entry}"

    # Default query path if not specified
    if [ -z "${query_path}" ]; then
        query_path="queries/highlights.scm"
    fi

    output="${name}-vendored.scm"

    echo "Fetching ${name} (${branch}) from ${repo}..."

    url="https://raw.githubusercontent.com/${repo}/${branch}/${query_path}"
    tmpfile=$(mktemp)
    cat > "${tmpfile}" << EOF
; Source: github.com/${repo} @ ${branch}
; Upstream: https://github.com/${repo}/blob/${branch}/${query_path}
;
; IMPORTANT: Upstream queries use "first match wins" but our highlighter uses
; "last match wins" (see MergeSpans). Queries may need reordering after fetching:
; general patterns (like @variable) should come BEFORE specific patterns (like @function).
;
; Check for LOCAL MODIFICATION comments below for any manual changes.

EOF

    if curl -sfL "${url}" >> "${tmpfile}" 2>/dev/null; then
        mv "${tmpfile}" "${output}"
        echo "  OK (from branch ${branch})"
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
