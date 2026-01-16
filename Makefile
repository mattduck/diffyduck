.PHONY: build install test check fmt lint clean update-golden

# Build the binary
build:
	go build -o diffyduck ./cmd/diffyduck/

# Install to GOPATH/bin
install:
	go install ./cmd/diffyduck/

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
	rm -f diffyduck
	go clean ./...

# Run the app on HEAD
run: build
	./diffyduck

# Show test coverage
cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"
