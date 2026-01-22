package sidebyside

// FoldLevel represents the fold state of a file in the diff view.
type FoldLevel int

const (
	FoldNormal   FoldLevel = iota // default: hunk-based view with gaps
	FoldExpanded                  // full file contents side-by-side
	FoldFolded                    // header line only
)

// NextLevel returns the next fold level in the cycle.
// Cycles: Normal -> Expanded -> Folded -> Normal
func (f FoldLevel) NextLevel() FoldLevel {
	return (f + 1) % 3
}

// String returns the human-readable name of the fold level.
func (f FoldLevel) String() string {
	switch f {
	case FoldNormal:
		return "Normal"
	case FoldExpanded:
		return "Expanded"
	case FoldFolded:
		return "Folded"
	default:
		return "Unknown"
	}
}

// LineType indicates the type of line for display purposes.
type LineType int

const (
	Empty   LineType = iota // blank line (other side has content)
	Context                 // unchanged line
	Added                   // new content
	Removed                 // deleted content
)

// Line represents one side of a side-by-side line.
type Line struct {
	Num     int      // line number (0 if empty)
	Content string   // text content
	Type    LineType // how to display this line
}

// LinePair represents a row in the side-by-side view.
// Old contains the original content, New contains the modified content.
type LinePair struct {
	Old Line
	New Line
}

// FilePair represents all the line pairs for a single file's diff.
type FilePair struct {
	OldPath string
	NewPath string
	Pairs   []LinePair

	// Fold state
	FoldLevel FoldLevel // current fold level (zero value = FoldNormal)

	// Cached full file content (populated lazily when expanded)
	OldContent []string // full old file lines (nil until fetched)
	NewContent []string // full new file lines (nil until fetched)

	// Truncation from diff parsing
	Truncated    bool // true if diff lines were truncated due to limit
	OldTruncated bool // true if old (left) side was truncated
	NewTruncated bool // true if new (right) side was truncated

	// Truncation from content fetching (expanded view)
	ContentTruncated    bool // true if fetched content was truncated due to limit (legacy, use per-side)
	OldContentTruncated bool // true if old content was truncated when fetched
	NewContentTruncated bool // true if new content was truncated when fetched

	// Stats (accurate even if truncated)
	TotalAdded   int // total added lines from diff
	TotalRemoved int // total removed lines from diff
}

// HasContent returns true if full file content has been loaded.
func (fp FilePair) HasContent() bool {
	return fp.OldContent != nil || fp.NewContent != nil
}
