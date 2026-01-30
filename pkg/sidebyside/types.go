package sidebyside

import (
	"fmt"
	"math"
	"time"
)

// FoldLevel represents the fold state of a file in the diff view.
type FoldLevel int

const (
	FoldNormal   FoldLevel = iota // part-expanded: structural diff preview
	FoldExpanded                  // full-expanded: hunk-based diff view
	FoldFolded                    // folded: header line only
)

// NextLevel returns the next fold level in the cycle.
// Cycles: Normal -> Expanded -> Folded -> Normal
// (structural diff -> hunks -> header only -> structural diff)
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

	// Rename/copy metadata from git
	IsRename   bool // true if "rename from/to" was present
	IsCopy     bool // true if "copy from/to" was present
	Similarity int  // similarity index percentage (0-100), -1 if not present

	// Binary file indicator
	IsBinary bool // true if "Binary files ... differ" was present
}

// HasContent returns true if full file content has been loaded.
func (fp FilePair) HasContent() bool {
	return fp.OldContent != nil || fp.NewContent != nil
}

// CommitFoldLevel represents the fold state of a commit in the view.
type CommitFoldLevel int

const (
	CommitFolded   CommitFoldLevel = iota // summary line only (sha, message preview, file count)
	CommitNormal                          // file headers visible, hunks shown per-file fold level
	CommitExpanded                        // all files expanded (full diffs visible)
)

// NextLevel returns the next fold level in the cycle.
// Cycles: Folded -> Normal -> Expanded -> Folded
func (c CommitFoldLevel) NextLevel() CommitFoldLevel {
	return (c + 1) % 3
}

// String returns the human-readable name of the commit fold level.
func (c CommitFoldLevel) String() string {
	switch c {
	case CommitFolded:
		return "Folded"
	case CommitNormal:
		return "Normal"
	case CommitExpanded:
		return "Expanded"
	default:
		return "Unknown"
	}
}

// CommitInfo contains metadata about a commit.
// All fields are optional - for plain diffs without commit context,
// this will be empty.
type CommitInfo struct {
	SHA     string // full commit hash (empty for plain diffs)
	Author  string // author name
	Email   string // author email
	Date    string // commit date as string (format varies)
	Subject string // first line of commit message
	Body    string // rest of commit message (may be empty)
}

// ShortSHA returns the first 7 characters of the SHA, or empty if no SHA.
func (c CommitInfo) ShortSHA() string {
	if len(c.SHA) >= 7 {
		return c.SHA[:7]
	}
	return c.SHA
}

// HasMetadata returns true if any commit metadata is present.
func (c CommitInfo) HasMetadata() bool {
	return c.SHA != "" || c.Author != "" || c.Subject != ""
}

// DateParts holds the segments of a formatted commit date for styled rendering.
type DateParts struct {
	Day    string // e.g. "Wednesday, " — dim
	Date   string // e.g. "Jan 6th 15:03" — bright
	Offset string // e.g. " +00:00" — dim
	Ago    string // e.g. " (3 days ago)" — dim
}

// Plain returns all parts concatenated without styling.
func (d DateParts) Plain() string {
	return d.Day + d.Date + d.Offset + d.Ago
}

// FormattedDate returns the commit date as a plain string (for header width
// calculations and fallback). Prefer FormattedDateParts for styled rendering.
func (c CommitInfo) FormattedDate(now time.Time) string {
	return c.FormattedDateParts(now).Plain()
}

// dateFormats lists formats to try when parsing commit dates.
var dateFormats = []string{
	time.RFC3339,
	"2006-01-02T15:04:05Z07:00",
	"Mon Jan 2 15:04:05 2006 -0700",
	"2006-01-02",
}

// FormattedDateParts returns the commit date split into segments for styled
// rendering: day (dim), date (bright), offset (dim), relative time (dim).
func (c CommitInfo) FormattedDateParts(now time.Time) DateParts {
	if c.Date == "" {
		return DateParts{Date: "details"}
	}
	var t time.Time
	var err error
	for _, layout := range dateFormats {
		t, err = time.Parse(layout, c.Date)
		if err == nil {
			break
		}
	}
	if err != nil {
		return DateParts{Date: c.Date}
	}

	_, offset := t.Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	tzHours := offset / 3600
	tzMins := (offset % 3600) / 60

	return DateParts{
		Day:    fmt.Sprintf("%s, ", t.Weekday()),
		Date:   fmt.Sprintf("%s %d%s %02d:%02d", t.Month().String()[:3], t.Day(), daySuffix(t.Day()), t.Hour(), t.Minute()),
		Offset: fmt.Sprintf(" %s%02d:%02d", sign, tzHours, tzMins),
		Ago:    fmt.Sprintf(" (%s)", relativeTime(now.Sub(t))),
	}
}

// daySuffix returns the ordinal suffix for a day number.
func daySuffix(day int) string {
	switch {
	case day == 11 || day == 12 || day == 13:
		return "th"
	case day%10 == 1:
		return "st"
	case day%10 == 2:
		return "nd"
	case day%10 == 3:
		return "rd"
	default:
		return "th"
	}
}

// relativeTime returns a human-readable relative time string.
func relativeTime(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	seconds := d.Seconds()
	switch {
	case seconds < 60:
		n := int(math.Round(seconds))
		if n == 1 {
			return "1 second ago"
		}
		return fmt.Sprintf("%d seconds ago", n)
	case seconds < 3600:
		n := int(math.Round(seconds / 60))
		if n == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", n)
	case seconds < 86400:
		n := int(math.Round(seconds / 3600))
		if n == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", n)
	case seconds < 86400*30:
		n := int(math.Round(seconds / 86400))
		if n == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", n)
	case seconds < 86400*365:
		n := int(math.Round(seconds / (86400 * 30)))
		if n == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", n)
	default:
		n := int(math.Round(seconds / (86400 * 365)))
		if n == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", n)
	}
}

// CommitSet represents a single commit or diff-set containing files.
// For log view, there are multiple CommitSets.
// For diff view, there is one CommitSet (possibly with empty CommitInfo).
type CommitSet struct {
	// Commit metadata (may be empty for plain diffs)
	Info CommitInfo

	// Files in this commit/diff
	Files []FilePair

	// Fold state for this commit
	FoldLevel CommitFoldLevel

	// Loading state
	FilesLoaded bool // true once files have been parsed (for async loading)
	StatsLoaded bool // true once per-file stats have been loaded (for progressive loading)

	// Truncation
	TruncatedFileCount int // number of files omitted due to limit

	// Cached stats (sum of all files, avoids recomputing in render loop)
	TotalAdded   int
	TotalRemoved int
}
