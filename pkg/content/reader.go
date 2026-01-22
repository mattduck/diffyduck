package content

import (
	"bufio"
	"io"

	"github.com/user/diffyduck/pkg/diff"
)

// ReadLimitedLines reads lines from a reader with limits applied.
// It stops reading when any of these limits is reached:
// - MaxLinesPerFile (10000 lines)
// - MaxContentBytes (1MB total bytes read)
//
// Returns the lines, whether truncation occurred, and any error.
// Truncation is true if we hit any limit (bytes or lines).
func ReadLimitedLines(r io.Reader) ([]string, bool, error) {
	return ReadLimitedLinesWithLimits(r, diff.MaxLinesPerFile, diff.MaxContentBytes)
}

// ReadLimitedLinesWithLimits reads lines with custom limits.
// This is useful for testing with smaller limits.
func ReadLimitedLinesWithLimits(r io.Reader, maxLines, maxBytes int) ([]string, bool, error) {
	var lines []string
	truncated := false

	// Use a limited reader to cap total bytes
	limitedReader := &limitedReader{r: r, remaining: maxBytes}
	scanner := bufio.NewScanner(limitedReader)

	// Set a large buffer for scanning (to handle long lines)
	buf := make([]byte, 64*1024) // 64KB buffer
	scanner.Buffer(buf, maxBytes)

	for scanner.Scan() {
		if len(lines) >= maxLines {
			truncated = true
			break
		}

		lines = append(lines, scanner.Text())

		// Check if we hit the byte limit
		if limitedReader.hitLimit {
			truncated = true
			break
		}
	}

	if err := scanner.Err(); err != nil {
		// If we hit the byte limit, that's not an error condition
		if limitedReader.hitLimit {
			truncated = true
			return lines, truncated, nil
		}
		return lines, truncated, err
	}

	return lines, truncated, nil
}

// limitedReader wraps an io.Reader and stops after reading maxBytes.
type limitedReader struct {
	r         io.Reader
	remaining int
	hitLimit  bool
}

func (lr *limitedReader) Read(p []byte) (n int, err error) {
	if lr.remaining <= 0 {
		lr.hitLimit = true
		return 0, io.EOF
	}

	if len(p) > lr.remaining {
		p = p[:lr.remaining]
	}

	n, err = lr.r.Read(p)
	lr.remaining -= n

	if lr.remaining <= 0 {
		lr.hitLimit = true
	}

	return n, err
}
