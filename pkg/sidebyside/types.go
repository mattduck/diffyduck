package sidebyside

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
type LinePair struct {
	Left  Line
	Right Line
}

// FilePair represents all the line pairs for a single file's diff.
type FilePair struct {
	OldPath string
	NewPath string
	Pairs   []LinePair
}
