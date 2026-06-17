.PHONY: build install test check fmt lint clean update-golden fetch-queries bootstrap cgo-free

# Build the binaries (set VERSION to inject a version string)
VERSION ?=
LDFLAGS := $(if $(VERSION),-ldflags "-X main.version=$(VERSION)",)

# All three frontends: the diff TUI (dfd), the ticket CLI (tdb), and the
# reviewparrot linter (rpt).
BINARIES := dfd tdb rpt

build:
	@for b in $(BINARIES); do echo "building $$b"; go build $(LDFLAGS) -o $$b ./cmd/$$b/ || exit 1; done

# Install to GOPATH/bin
install:
	go install $(LDFLAGS) ./cmd/dfd/ ./cmd/tdb/ ./cmd/rpt/

# Run all tests
test:
	go test ./...

# Run tests with verbose output
test-v:
	go test -v ./...

# Update golden files after intentional view changes
update-golden:
	go test ./internal/tui/... -update

# Run all checks (fmt, vet, lint, cgo-free, test)
check: fmt-check vet lint cgo-free test

# tdb and rpt must build without cgo: tree-sitter (cgo) is a dfd-only dependency.
# This is a regression gate — neither tool should re-acquire a cgo import.
cgo-free:
	CGO_ENABLED=0 go build -o /dev/null ./cmd/tdb/ ./cmd/rpt/

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
	rm -f $(BINARIES)
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
# Download generated parser files that are too large to commit (e.g. SQL ~38MB).
# Run this once after cloning before building.
bootstrap:
	go generate ./pkg/highlight/grammars/...

fetch-queries:
	./pkg/highlight/queries/fetch_queries.sh
