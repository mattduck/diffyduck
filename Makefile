.PHONY: build install test check fmt lint clean update-golden fetch-queries

# Build the binary
build:
	go build -o dfd ./cmd/dfd/

# Install to GOPATH/bin
install:
	go install ./cmd/dfd/

# Run all tests
test:
	go test ./...

# Run tests with verbose output
test-v:
	go test -v ./...

# Update golden files after intentional view changes
update-golden:
	go test ./internal/tui/... -update

# Run all checks (fmt, vet, lint, test)
check: fmt-check vet lint test

# Check formatting (fails if files need formatting)
fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:"; gofmt -l .; exit 1)

# Format code
fmt:
	gofmt -w .

# Run go vet
vet:
	go vet ./...

# Run staticcheck if available
lint:
	@which staticcheck > /dev/null && staticcheck ./... || echo "staticcheck not installed, skipping"

# Clean build artifacts
clean:
	rm -f dfd
	go clean ./...

# Run the app on HEAD
run: build
	./dfd

# Show test coverage
cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Fetch syntax highlighting queries from upstream tree-sitter grammar repos.
# This downloads .scm query files for all supported languages.
# NOTE: We use upstream repos, NOT nvim-treesitter (which has Lua-specific predicates).
# WARNING: This overwrites local modifications! After running:
#   1. Check 'git diff pkg/highlight/queries/' for changes
#   2. Reorder patterns if needed (general before specific for "last match wins")
#   3. Re-apply any LOCAL MODIFICATION sections
# See pkg/highlight/queries/fetch_queries.sh for details.
fetch-queries:
	./pkg/highlight/queries/fetch_queries.sh
