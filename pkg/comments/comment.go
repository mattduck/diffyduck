// Package comments provides persistence for diff comments using git refs.
//
// Comments are stored in a tree structure under refs/dfd/comments, similar to
// how git notes works. An index maps file paths to comment IDs for efficient
// lookup, and each comment is stored as a blob in patch format with metadata.
//
// TODO: Future support for remote fetch/merge of the comments ref for collaboration.
package comments

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RefPath is the git ref where comments are stored.
const RefPath = "refs/dfd/comments"

// Comment represents a persisted comment with its metadata and content.
type Comment struct {
	// ID is the unique identifier (unix timestamp in milliseconds).
	ID string

	// Text is the actual comment content (may be multi-line).
	Text string

	// File is the path of the file this comment is attached to.
	File string

	// Line is the original line number when the comment was created.
	Line int

	// Anchor is the hash of (line content + 2 lines above + 2 lines below).
	Anchor string

	// Context stores the original line content and surrounding context.
	Context LineContext

	// Created is when the comment was first created.
	Created time.Time

	// Updated is when the comment was last modified.
	Updated time.Time

	// CommitSHA is the commit that was being viewed when the comment was created.
	CommitSHA string

	// Branch is the branch that was checked out when the comment was created.
	Branch string

	// BranchHead is the tip commit of the branch when the comment was created.
	BranchHead string

	// Resolved marks the comment as resolved (addressed/done).
	Resolved bool

	// Author is an optional identifier for who created the comment (e.g. agent name).
	Author string
}

// LineContext stores the original line and its surrounding context for matching.
type LineContext struct {
	// Line is the actual content of the commented line.
	Line string

	// Above contains up to 2 lines above the commented line.
	Above []string

	// Below contains up to 2 lines below the commented line.
	Below []string
}

// ComputeAnchor calculates a hash from the line context for matching.
func (lc LineContext) ComputeAnchor() string {
	var parts []string
	parts = append(parts, lc.Above...)
	parts = append(parts, lc.Line)
	parts = append(parts, lc.Below...)
	content := strings.Join(parts, "\n")
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes (32 hex chars)
}

// NewID generates a new comment ID based on the current time.
func NewID() string {
	return strconv.FormatInt(time.Now().UnixMilli(), 10)
}

// IsStandalone returns true if the comment has no file attachment.
func (c *Comment) IsStandalone() bool {
	return c.File == ""
}

// ShortSuffixes computes the shortest unique suffix for each comment ID.
// The minimum suffix length is 3 characters (or the full ID if shorter).
func ShortSuffixes(ids []string) map[string]string {
	result := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return result
	}

	for _, id := range ids {
		maxN := len(id)
		minN := 3
		if minN > maxN {
			minN = maxN
		}
		var suffix string

		for n := minN; n <= maxN; n++ {
			start := maxN - n
			suffix = id[start:]
			unique := true
			for _, other := range ids {
				if other == id {
					continue
				}
				if strings.HasSuffix(other, suffix) {
					unique = false
					break
				}
			}
			if unique {
				break
			}
		}

		result[id] = suffix
	}
	return result
}

// isKnownMetadataField reports whether s starts with a known metadata key.
func isKnownMetadataField(s string) bool {
	return strings.HasPrefix(s, "CREATED:") ||
		strings.HasPrefix(s, "UPDATED:") ||
		strings.HasPrefix(s, "COMMIT:") ||
		strings.HasPrefix(s, "BRANCH:") ||
		strings.HasPrefix(s, "BRANCH_HEAD:") ||
		strings.HasPrefix(s, "HEAD:") ||
		strings.HasPrefix(s, "FILE:") ||
		strings.HasPrefix(s, "LINE:") ||
		strings.HasPrefix(s, "ANCHOR:") ||
		strings.HasPrefix(s, "AUTHOR:") ||
		strings.HasPrefix(s, "RESOLVED:")
}

// looksLikeMetadataField reports whether s looks like an unknown metadata
// field (e.g. "BRANCH_HEAD: abc123"). The key must be all uppercase letters
// and underscores, and the value must be a single token (no spaces) to
// distinguish from comment text like "FIX: use the correct buffer".
// Known fields are handled by isKnownMetadataField and don't go through
// this heuristic.
func looksLikeMetadataField(s string) bool {
	idx := strings.IndexByte(s, ':')
	if idx <= 0 {
		return false
	}
	key := s[:idx]
	for _, r := range key {
		if !((r >= 'A' && r <= 'Z') || r == '_') {
			return false
		}
	}
	// Require the value to be a single token — metadata values are
	// typically SHAs, timestamps, paths, or booleans, not prose.
	val := strings.TrimPrefix(s[idx+1:], " ")
	if val == "" {
		return true
	}
	return !strings.Contains(val, " ")
}

// Serialize converts a Comment to its blob format (patch with metadata).
func (c *Comment) Serialize() string {
	var b strings.Builder

	// Write patch header and context only for file-attached comments
	if c.File != "" {
		b.WriteString(fmt.Sprintf("--- a/%s\n", c.File))
		b.WriteString(fmt.Sprintf("+++ b/%s\n", c.File))

		// Calculate hunk header line numbers
		// We show 2 lines above, the line itself, and 2 lines below (up to 5 lines total)
		startLine := max(1, c.Line-len(c.Context.Above))
		totalLines := len(c.Context.Above) + 1 + len(c.Context.Below)
		b.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", startLine, totalLines, startLine, totalLines))

		// Write context lines above
		for _, line := range c.Context.Above {
			b.WriteString(" ")
			b.WriteString(line)
			b.WriteString("\n")
		}

		// Write the commented line (as addition to make it stand out)
		b.WriteString("+")
		b.WriteString(c.Context.Line)
		b.WriteString("\n")

		// Write context lines below
		for _, line := range c.Context.Below {
			b.WriteString(" ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Write ID first so it's easy to find
	b.WriteString(fmt.Sprintf("# ID: %s\n", c.ID))

	// Write metadata before comment text so that unknown fields added by
	// newer versions appear in the metadata section (before COMMENT:) and
	// are naturally skipped by older parsers that don't recognise them.
	b.WriteString(fmt.Sprintf("# CREATED: %s\n", c.Created.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("# UPDATED: %s\n", c.Updated.Format(time.RFC3339)))
	if c.CommitSHA != "" {
		b.WriteString(fmt.Sprintf("# COMMIT: %s\n", c.CommitSHA))
	}
	if c.Branch != "" {
		b.WriteString(fmt.Sprintf("# BRANCH: %s\n", c.Branch))
	}
	if c.BranchHead != "" {
		b.WriteString(fmt.Sprintf("# BRANCH_HEAD: %s\n", c.BranchHead))
	}
	if c.Author != "" {
		b.WriteString(fmt.Sprintf("# AUTHOR: %s\n", c.Author))
	}
	b.WriteString(fmt.Sprintf("# RESOLVED: %t\n", c.Resolved))
	b.WriteString(fmt.Sprintf("# FILE: %s\n", c.File))
	b.WriteString(fmt.Sprintf("# LINE: %d\n", c.Line))
	b.WriteString(fmt.Sprintf("# ANCHOR: %s\n", c.Anchor))

	// Write comment text last with #| prefix to distinguish from metadata
	b.WriteString("# COMMENT:\n")
	for _, line := range strings.Split(c.Text, "\n") {
		b.WriteString("#| ")
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

// ParseComment parses a comment blob into a Comment struct.
func ParseComment(id string, data string) (*Comment, error) {
	c := &Comment{ID: id}

	lines := strings.Split(data, "\n")
	var inComment bool
	var commentLines []string
	var contextAbove []string
	var contextBelow []string
	var commentedLine string
	var seenCommentedLine bool

	for _, line := range lines {
		// Parse comment text (checked first so metadata-like lines inside
		// the comment section aren't consumed by the handlers below).
		if line == "# COMMENT:" {
			inComment = true
			continue
		}
		// New format: #| prefix is unambiguously comment text
		if inComment && strings.HasPrefix(line, "#| ") {
			commentLines = append(commentLines, strings.TrimPrefix(line, "#| "))
			continue
		}
		if inComment && strings.HasPrefix(line, "# ") {
			// Old format: # prefix for comment text. Check if this is a
			// known metadata line (ends the comment section).
			rest := strings.TrimPrefix(line, "# ")
			if isKnownMetadataField(rest) {
				inComment = false
				// Trim trailing lines that look like unknown metadata
				// fields (e.g. BRANCH_HEAD:) so they don't leak into
				// the displayed comment text. Only trim from the end
				// to avoid false-positives on comment text like
				// "FIX: something" in the middle of the comment.
				for len(commentLines) > 0 && looksLikeMetadataField(commentLines[len(commentLines)-1]) {
					commentLines = commentLines[:len(commentLines)-1]
				}
				// Fall through to the metadata handlers below so this
				// line's value is parsed.
			} else {
				commentLines = append(commentLines, rest)
				continue
			}
		}

		// Parse metadata lines
		if strings.HasPrefix(line, "# ID: ") {
			// ID is already set from the blob name; skip this line.
			continue
		}
		if strings.HasPrefix(line, "# CREATED: ") {
			t, err := time.Parse(time.RFC3339, strings.TrimPrefix(line, "# CREATED: "))
			if err == nil {
				c.Created = t
			}
			continue
		}
		if strings.HasPrefix(line, "# UPDATED: ") {
			t, err := time.Parse(time.RFC3339, strings.TrimPrefix(line, "# UPDATED: "))
			if err == nil {
				c.Updated = t
			}
			continue
		}
		if strings.HasPrefix(line, "# COMMIT: ") {
			c.CommitSHA = strings.TrimPrefix(line, "# COMMIT: ")
			continue
		}
		if strings.HasPrefix(line, "# BRANCH: ") {
			c.Branch = strings.TrimPrefix(line, "# BRANCH: ")
			continue
		}
		if strings.HasPrefix(line, "# BRANCH_HEAD: ") {
			c.BranchHead = strings.TrimPrefix(line, "# BRANCH_HEAD: ")
			continue
		}
		if strings.HasPrefix(line, "# HEAD: ") {
			// Backward compat with old format
			c.BranchHead = strings.TrimPrefix(line, "# HEAD: ")
			continue
		}
		if strings.HasPrefix(line, "# FILE: ") {
			c.File = strings.TrimPrefix(line, "# FILE: ")
			continue
		}
		if strings.HasPrefix(line, "# LINE: ") {
			n, err := strconv.Atoi(strings.TrimPrefix(line, "# LINE: "))
			if err == nil {
				c.Line = n
			}
			continue
		}
		if strings.HasPrefix(line, "# ANCHOR: ") {
			c.Anchor = strings.TrimPrefix(line, "# ANCHOR: ")
			continue
		}
		if strings.HasPrefix(line, "# AUTHOR: ") {
			c.Author = strings.TrimPrefix(line, "# AUTHOR: ")
			continue
		}
		if strings.HasPrefix(line, "# RESOLVED: ") {
			val := strings.TrimPrefix(line, "# RESOLVED: ")
			switch val {
			case "true":
				c.Resolved = true
			case "false":
				c.Resolved = false
			default:
				return nil, fmt.Errorf("invalid RESOLVED value %q: expected true or false", val)
			}
			continue
		}

		// Parse diff context lines
		if strings.HasPrefix(line, " ") && !seenCommentedLine {
			// Context line before the commented line
			contextAbove = append(contextAbove, strings.TrimPrefix(line, " "))
			continue
		}
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			// The commented line itself
			commentedLine = strings.TrimPrefix(line, "+")
			seenCommentedLine = true
			continue
		}
		if strings.HasPrefix(line, " ") && seenCommentedLine {
			// Context line after the commented line
			contextBelow = append(contextBelow, strings.TrimPrefix(line, " "))
			continue
		}
	}

	c.Text = strings.Join(commentLines, "\n")
	c.Context = LineContext{
		Line:  commentedLine,
		Above: contextAbove,
		Below: contextBelow,
	}

	return c, nil
}
