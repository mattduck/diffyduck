package branches

import (
	"fmt"
	"strings"
	"time"
)

// ANSI color helpers
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m" // fg=1
	colorGreen  = "\033[32m" // fg=2
	colorYellow = "\033[33m" // fg=3
	colorCyan   = "\033[36m" // fg=6
	colorWhite  = "\033[37m" // fg=7
)

// Render formats a forest of BranchNodes as a tree-view string.
func Render(roots []*BranchNode, verbose bool) string {
	return RenderAt(roots, verbose, time.Now())
}

// RenderAt formats a forest of BranchNodes as a tree-view string,
// using the given time as "now" for relative date calculation.
func RenderAt(roots []*BranchNode, verbose bool, now time.Time) string {
	if len(roots) == 0 {
		return ""
	}

	// First pass: collect all lines to compute column widths
	type line struct {
		nameCol   string // tree prefix + branch name (plain, for width calc)
		countsCol string // "+N -M" or "" (plain, for width calc)
		isHead    bool
		virtual   bool
		behind    int
		sha       string
		dateStr   string
		subject   string
		author    string
	}

	var lines []line

	var walk func(node *BranchNode, prefix string, isLast bool, isRoot bool)
	walk = func(node *BranchNode, prefix string, isLast bool, isRoot bool) {
		// Build name column (plain text for width calculation)
		var nameCol string
		displayName := node.Name
		if node.Virtual {
			displayName = "(" + node.SHA + ")"
		}

		if isRoot {
			if node.IsHead {
				nameCol = "* " + displayName
			} else {
				nameCol = "  " + displayName
			}
		} else {
			connector := "├─ "
			if isLast {
				connector = "└─ "
			}
			if node.IsHead {
				nameCol = prefix + connector + "* " + displayName
			} else {
				nameCol = prefix + connector + displayName
			}
		}

		// Build counts column
		var countsCol string
		if !isRoot {
			countsCol = fmt.Sprintf("+%d", node.Ahead)
			if node.Behind > 0 {
				countsCol += fmt.Sprintf(" -%d", node.Behind)
			}
		}

		// Virtual nodes only show name (with SHA) and counts
		sha := node.SHA
		dateStr := relativeTime(node.Date, now)
		subject := node.Subject
		author := node.Author
		if node.Virtual {
			sha = ""
			dateStr = ""
			subject = ""
			author = ""
		}

		lines = append(lines, line{
			nameCol:   nameCol,
			countsCol: countsCol,
			isHead:    node.IsHead,
			virtual:   node.Virtual,
			behind:    node.Behind,
			sha:       sha,
			dateStr:   dateStr,
			subject:   subject,
			author:    author,
		})

		// Recurse into children
		childPrefix := prefix
		if !isRoot {
			if isLast {
				childPrefix = prefix + "   "
			} else {
				childPrefix = prefix + "│  "
			}
		}
		for i, child := range node.Children {
			walk(child, childPrefix, i == len(node.Children)-1, false)
		}
	}

	for i, root := range roots {
		if i > 0 {
			// Blank line between independent trees
			lines = append(lines, line{})
		}
		walk(root, "  ", true, true)
	}

	// Compute max widths
	maxName := 0
	maxCounts := 0
	maxDate := 0
	for _, l := range lines {
		if len(l.nameCol) > maxName {
			maxName = len(l.nameCol)
		}
		if len(l.countsCol) > maxCounts {
			maxCounts = len(l.countsCol)
		}
		if len(l.dateStr) > maxDate {
			maxDate = len(l.dateStr)
		}
	}

	// Second pass: render with padding and colors
	var sb strings.Builder
	for _, l := range lines {
		if l.nameCol == "" && l.sha == "" {
			sb.WriteByte('\n')
			continue
		}

		// Name column: green for HEAD, dim for virtual, cyan for others
		nameColor := colorCyan
		if l.isHead {
			nameColor = colorGreen
		} else if l.virtual {
			nameColor = colorWhite
		}

		// Counts column: red if behind, default otherwise
		countsColor := colorReset
		if l.behind > 0 {
			countsColor = colorRed
		}

		if verbose {
			fmt.Fprintf(&sb, "%s%-*s%s  %s%-*s%s  %s%s%s  %s%-*s%s  %s  %s%s%s\n",
				nameColor, maxName, l.nameCol, colorReset,
				countsColor, maxCounts, l.countsCol, colorReset,
				colorYellow, l.sha, colorReset,
				colorWhite, maxDate, l.dateStr, colorReset,
				l.subject,
				colorCyan, l.author, colorReset,
			)
		} else {
			fmt.Fprintf(&sb, "%s%-*s%s  %s%-*s%s  %s%s%s  %s%s%s\n",
				nameColor, maxName, l.nameCol, colorReset,
				countsColor, maxCounts, l.countsCol, colorReset,
				colorYellow, l.sha, colorReset,
				colorWhite, l.dateStr, colorReset,
			)
		}
	}

	return sb.String()
}

// relativeTime formats a time as a human-readable relative string.
func relativeTime(t time.Time, now time.Time) string {
	if t.IsZero() {
		return ""
	}

	d := now.Sub(t)
	if d < 0 {
		d = -d
	}

	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw ago", int(d.Hours()/(24*7)))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy ago", int(d.Hours()/(24*365)))
	}
}
