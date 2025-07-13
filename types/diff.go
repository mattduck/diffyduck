package types

import "github.com/sergi/go-diff/diffmatchpatch"

// DiffSegment represents a segment of text with its diff operation type
type DiffSegment struct {
	Text string
	Type diffmatchpatch.Operation
}
