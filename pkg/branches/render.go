package branches

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// ANSI color helpers
const (
	colorReset     = "\033[0m"
	colorRed       = "\033[31m" // fg=1
	colorGreen     = "\033[32m" // fg=2
	colorYellow    = "\033[33m" // fg=3
	colorCyan      = "\033[36m" // fg=6
	colorWhite     = "\033[37m" // fg=7
	colorBrightBlk = "\033[90m" // fg=8

	underlineOn  = "\033[4m"
	underlineOff = "\033[24m"
)

// Render formats a forest of BranchNodes as a tree-view string.
// dirtyBranches marks branch names whose worktrees have uncommitted changes.
func Render(roots []*BranchNode, verbose bool, dirtyBranches map[string]bool) string {
	return RenderAt(roots, verbose, time.Now(), dirtyBranches)
}

// RenderAt formats a forest of BranchNodes as a tree-view string,
// using the given time as "now" for relative date calculation.
// dirtyBranches marks branch names whose worktrees have uncommitted changes.
func RenderAt(roots []*BranchNode, verbose bool, now time.Time, dirtyBranches map[string]bool) string {
	if len(roots) == 0 {
		return ""
	}

	// First pass: collect all lines to compute column widths
	type upstreamEntry struct {
		text    string // plain text for this upstream (e.g. "origin/main =")
		color   string // per-upstream color
		isHead  bool   // true if this upstream matches the HEAD branch
		headRef string // the upstream name to underline (when isHead)
	}
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
		upstreams []upstreamEntry // per-upstream info for individual coloring
		headRef   string          // specific HEAD branch name to underline
	}

	var lines []line

	var walk func(node *BranchNode, prefix string, isFirst bool, isRoot bool)
	walk = func(node *BranchNode, prefix string, isFirst bool, isRoot bool) {
		// Build name column (plain text for width calculation)
		var nameCol string
		displayName := node.Name
		if node.Virtual {
			displayName = "(" + node.SHA + ")"
		}

		if node.IsHead {
			displayName = insertStarBeforeBranch(displayName, node.HeadRef)
		}
		if len(dirtyBranches) > 0 && !node.Virtual {
			displayName = insertTildeForDirty(displayName, dirtyBranches)
		}

		if isRoot {
			nameCol = displayName
		} else {
			connector := "├─ "
			if isFirst {
				connector = "┌─ "
			}
			nameCol = prefix + connector + displayName
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

		// Build per-upstream entries
		var upstreams []upstreamEntry
		for _, u := range node.Upstreams {
			text := u.Name
			color := colorBrightBlk // default: synced or ahead
			if u.Gone {
				text += " gone"
				color = colorRed
			} else if u.Ahead == 0 && u.Behind == 0 {
				text += " ="
			} else {
				if u.Ahead > 0 {
					text += fmt.Sprintf(" ↑%d", u.Ahead)
				}
				if u.Behind > 0 {
					text += fmt.Sprintf(" ↓%d", u.Behind)
					color = colorRed
				}
			}
			upstreams = append(upstreams, upstreamEntry{text: text, color: color})
		}

		// Recurse into children first (bottom-up: children appear above parent)
		childPrefix := prefix
		if !isRoot {
			if isFirst {
				childPrefix = prefix + "   "
			} else {
				childPrefix = prefix + "│  "
			}
		}
		for i, child := range node.Children {
			walk(child, childPrefix, i == 0, false)
		}

		// Mark the upstream entry matching the HEAD branch for underlining.
		if node.HeadRef != "" {
			for i := range upstreams {
				if strings.HasSuffix(upstreams[i].text, "/"+node.HeadRef+" =") ||
					strings.Contains(upstreams[i].text, "/"+node.HeadRef+" ") ||
					strings.HasSuffix(upstreams[i].text, "/"+node.HeadRef) {
					// Extract just the upstream name (first space-separated token)
					name := upstreams[i].text
					if idx := strings.Index(name, " "); idx >= 0 {
						name = name[:idx]
					}
					upstreams[i].isHead = true
					upstreams[i].headRef = name
					break
				}
			}
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
			upstreams: upstreams,
			headRef:   node.HeadRef,
		})
	}

	for i, root := range roots {
		if i > 0 {
			// Blank line between independent trees
			lines = append(lines, line{})
		}
		walk(root, "", true, true)
	}

	// Compute max widths
	maxName := 0
	maxCounts := 0
	maxDate := 0
	for _, l := range lines {
		if w := utf8.RuneCountInString(l.nameCol); w > maxName {
			maxName = w
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

		// (upstream colors are per-entry, applied below)

		// Apply underline to HEAD branch name within the name column
		styledName := l.nameCol
		if l.headRef != "" {
			styledName = underlineInString(styledName, l.headRef)
		}
		namePad := maxName - utf8.RuneCountInString(l.nameCol)

		if verbose {
			fmt.Fprintf(&sb, "%s%s%*s%s  %s%-*s%s  %s%s%s  %s%-*s%s  %s  %s%s%s",
				nameColor, styledName, namePad, "", colorReset,
				countsColor, maxCounts, l.countsCol, colorReset,
				colorYellow, l.sha, colorReset,
				colorWhite, maxDate, l.dateStr, colorReset,
				l.subject,
				colorCyan, l.author, colorReset,
			)
		} else {
			fmt.Fprintf(&sb, "%s%s%*s%s  %s%-*s%s  %s%s%s  %s%s%s",
				nameColor, styledName, namePad, "", colorReset,
				countsColor, maxCounts, l.countsCol, colorReset,
				colorYellow, l.sha, colorReset,
				colorWhite, l.dateStr, colorReset,
			)
		}
		if len(l.upstreams) > 0 {
			sb.WriteString("  ")
			for k, u := range l.upstreams {
				if k > 0 {
					sb.WriteString(colorReset + ", ")
				}
				text := u.text
				if u.isHead && u.headRef != "" {
					text = underlineInString(text, u.headRef)
				}
				fmt.Fprintf(&sb, "%s%s", u.color, text)
			}
			sb.WriteString(colorReset)
		}
		sb.WriteByte('\n')
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
		return fmt.Sprintf("%dM ago", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy ago", int(d.Hours()/(24*365)))
	}
}

// insertStarBeforeBranch places a "*" before the HEAD branch name within
// a possibly comma-separated display name (e.g. "a, main, z" → "a, *main, z").
// Falls back to prepending "*" when headRef is empty or not found.
func insertStarBeforeBranch(displayName, headRef string) string {
	if headRef == "" {
		return "*" + displayName
	}
	names := strings.Split(displayName, ", ")
	for i, name := range names {
		if name == headRef {
			names[i] = "*" + name
			return strings.Join(names, ", ")
		}
	}
	return "*" + displayName
}

// insertTildeForDirty appends "~" to each branch name in a possibly
// comma-separated display name that appears in the dirty set.
// Handles names already prefixed with "*" (HEAD marker).
func insertTildeForDirty(displayName string, dirty map[string]bool) string {
	if len(dirty) == 0 {
		return displayName
	}
	names := strings.Split(displayName, ", ")
	changed := false
	for i, name := range names {
		clean := strings.TrimPrefix(name, "*")
		if dirty[clean] {
			names[i] = name + "~"
			changed = true
		}
	}
	if !changed {
		return displayName
	}
	return strings.Join(names, ", ")
}

// underlineInString inserts underline ANSI codes around the first
// occurrence of name in s.
func underlineInString(s, name string) string {
	if name == "" {
		return s
	}
	idx := strings.Index(s, name)
	if idx < 0 {
		return s
	}
	return s[:idx] + underlineOn + name + underlineOff + s[idx+len(name):]
}
