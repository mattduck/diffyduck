package ticketcli

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/mattduck/diffyduck/pkg/ticketdb"
)

// Options holds the parsed inputs for a single comment/note CLI invocation,
// plus the rendering dependencies (styles and optional highlighter) supplied by
// the host binary.
type Options struct {
	// Note is true when the command was invoked via the "note" alias, which
	// scopes listing/editing to standalone comments.
	Note bool

	Sub     string // list, edit, add, resolve, unresolve
	ID      string // comment ID/suffix for edit or list lookup
	Kind    string // list filter: comment, note, all
	Status  string // list filter: unresolved (default), resolved, all
	N       int    // -n count: positive=newest, negative=oldest, 0=uncapped
	NSet    bool   // true if -n was explicitly passed
	Verbose bool   // -v: block output
	Raw     bool   // --raw: serialized blob output

	AllBranches bool   // --all-branches
	Branch      string // --branch / -b filter ("." resolves to current branch)

	Resolved *bool // --resolved=true/false (edit only)

	Since     string // --since duration (list only)
	Author    string // --author value (add: set, list: filter)
	AuthorSet bool   // true if --author was explicitly passed
	File      string // --file filter (list only)
	Grep      string // --grep filter (list only)
	Marker    string // --marker (add: set; list: filter) — marker keyword tag
	Type      string // --type (add: set; list: filter) — conventional-commit type
	Scope     string // --scope (add: set; list: filter) — scope/code identifier

	AddTarget  string // file:line positional arg (add)
	AddMessage string // -m message (add)
	AddRef     string // --ref commit/branch/tag (add)

	// Styles drives colored output. The zero value renders without color.
	Styles CommentListStyles
	// Highlighter optionally syntax-highlights a comment's code context in block
	// output. When nil, context is rendered as plain text (the cgo-free path).
	Highlighter ContextHighlighter
}

// ContextHighlighter optionally syntax-highlights a comment's code context.
// dfd supplies a tree-sitter-backed implementation; tdb passes nil. Keeping the
// dependency behind this interface is what allows tdb to build CGO_ENABLED=0.
type ContextHighlighter interface {
	// HighlightContext returns the comment's context lines (above + target +
	// below) in order, optionally with syntax highlighting applied.
	HighlightContext(c *ticketdb.Comment) []string
}

// FormatRelativeAge returns a compact relative age string:
// "0m", "34m", "2h", "3d", "2w", "3M", "1y".
func FormatRelativeAge(now, t time.Time) string {
	d := now.Sub(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Hour:
		m := int(d.Minutes())
		if m < 1 {
			return "0m"
		}
		return fmt.Sprintf("%dm", m)
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw", int(d.Hours()/(7*24)))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dM", int(d.Hours()/(30*24)))
	default:
		return fmt.Sprintf("%dy", int(d.Hours()/(365*24)))
	}
}

// parseSinceDuration parses a compact duration like "30m", "6h", "30d", "2w",
// "3M", "1y", or "all"/"" (no limit).
func parseSinceDuration(s string) (time.Duration, error) {
	if s == "" || s == "all" {
		return 0, nil
	}
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration %q: expected number + unit (m/h/d/w/M/y)", s)
	}
	numStr := s[:len(s)-1]
	unit := s[len(s)-1]
	n, err := strconv.Atoi(numStr)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid duration %q: expected positive number + unit (m/h/d/w/M/y)", s)
	}
	switch unit {
	case 'm':
		return time.Duration(n) * time.Minute, nil
	case 'h':
		return time.Duration(n) * time.Hour, nil
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	case 'M':
		return time.Duration(n) * 30 * 24 * time.Hour, nil
	case 'y':
		return time.Duration(n) * 365 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid duration unit %q: expected m, h, d, w, M, or y", string(unit))
	}
}

// editorCmd returns the user's preferred editor command ($VISUAL, then $EDITOR).
func editorCmd() string {
	if e := os.Getenv("VISUAL"); e != "" {
		return e
	}
	return os.Getenv("EDITOR")
}
