package diff

// LineType indicates whether a line is context, added, or removed.
type LineType int

const (
	Context LineType = iota
	Added
	Removed
)

// Limits to prevent performance issues with large diffs
const (
	MaxLineLength      = 300              // Maximum characters per line before truncation
	LineTruncationText = "[...truncated]" // Suffix appended to truncated lines
	MaxLinesPerFile    = 10000            // Maximum lines per file before truncation
	MaxFiles           = 1000             // Maximum number of files before truncation
)

// Line represents a single line in a diff hunk.
type Line struct {
	Type    LineType
	Content string
}

// Hunk represents a single hunk in a diff, with line number info and lines.
type Hunk struct {
	OldStart int // starting line number in old file
	OldCount int // number of lines in old file
	NewStart int // starting line number in new file
	NewCount int // number of lines in new file
	Lines    []Line
}

// File represents a single file's diff.
type File struct {
	OldPath   string
	NewPath   string
	Hunks     []Hunk
	Truncated bool // True if lines were truncated due to MaxLinesPerFile limit
}

// Diff represents a complete diff, possibly spanning multiple files.
type Diff struct {
	Files              []File
	TruncatedFileCount int // Number of files omitted due to MaxFiles limit
}
