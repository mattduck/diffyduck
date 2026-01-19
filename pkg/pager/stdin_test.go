package pager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Note: IsStdinPipe() is difficult to unit test directly since it depends
// on the actual state of stdin. It's tested implicitly through integration
// tests and manual testing.

func TestReadStdin_StripsANSI(t *testing.T) {
	// This tests the contract that ReadStdin strips ANSI codes.
	// We can't easily test the actual stdin reading in a unit test,
	// but we can verify the stripping behavior through the StripANSI function
	// which ReadStdin uses internally.

	// Verify that StripANSI is used by checking colored input produces clean output
	colored := "\x1b[32m+added line\x1b[m"
	expected := "+added line"
	assert.Equal(t, expected, StripANSI(colored))
}

// TestIsStdinPipe_DocumentsBehavior documents the expected behavior.
// When run normally (go test), stdin is not a pipe, so this should return false.
// When run with piped input (echo "x" | go test), it would return true.
func TestIsStdinPipe_DocumentsBehavior(t *testing.T) {
	// In a normal test run, stdin is a terminal (or at least not a pipe)
	// This test documents the expected return value in test context
	result := IsStdinPipe()
	// We don't assert a specific value because it depends on how tests are run,
	// but we verify the function runs without error
	t.Logf("IsStdinPipe() = %v (depends on test execution context)", result)
}
