package diff

// LineType indicates whether a line is context, added, or removed.
type LineType int

const (
	Context LineType = iota
	Added
	Removed
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
	OldPath string
	NewPath string
	Hunks   []Hunk
}

// Diff represents a complete diff, possibly spanning multiple files.
type Diff struct {
	Files []File
}
