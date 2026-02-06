package comments

import (
	"strings"
)

// MatchResult represents where a comment was found in current file content.
type MatchResult struct {
	// Found is true if the comment's anchor was matched.
	Found bool

	// Line is the 1-based line number where the match was found.
	// Only valid if Found is true.
	Line int

	// Exact is true if the anchor hash matched exactly.
	// False means the line content matched but context changed.
	Exact bool
}

// ContextLines is the number of lines above and below to use for anchoring.
const ContextLines = 2

// FindCommentPosition searches for where a comment should appear in file content.
// It tries to match the comment's stored line content and context against the
// current file content.
//
// Returns MatchResult indicating if/where the comment was found.
func FindCommentPosition(c *Comment, fileLines []string) MatchResult {
	if c.Context.Line == "" {
		return MatchResult{Found: false}
	}

	// First, try exact anchor match at original position
	if c.Line > 0 && c.Line <= len(fileLines) {
		ctx := extractContext(fileLines, c.Line)
		if ctx.ComputeAnchor() == c.Anchor {
			return MatchResult{Found: true, Line: c.Line, Exact: true}
		}
	}

	// Search for exact anchor match elsewhere in file
	for i := 1; i <= len(fileLines); i++ {
		if i == c.Line {
			continue // Already checked
		}
		ctx := extractContext(fileLines, i)
		if ctx.ComputeAnchor() == c.Anchor {
			return MatchResult{Found: true, Line: i, Exact: true}
		}
	}

	// Fall back to line content match (context may have changed)
	// First check original position
	if c.Line > 0 && c.Line <= len(fileLines) {
		if fileLines[c.Line-1] == c.Context.Line {
			return MatchResult{Found: true, Line: c.Line, Exact: false}
		}
	}

	// Search for line content match, preferring closest to original position
	bestMatch := -1
	bestDistance := len(fileLines) + 1

	for i := 1; i <= len(fileLines); i++ {
		if fileLines[i-1] == c.Context.Line {
			distance := abs(i - c.Line)
			if distance < bestDistance {
				bestMatch = i
				bestDistance = distance
			}
		}
	}

	if bestMatch > 0 {
		return MatchResult{Found: true, Line: bestMatch, Exact: false}
	}

	return MatchResult{Found: false}
}

// ExtractContextForLine extracts the line content and surrounding context
// for a given line number in a file. Line numbers are 1-based.
func ExtractContextForLine(fileLines []string, lineNum int) LineContext {
	return extractContext(fileLines, lineNum)
}

// extractContext extracts LineContext for a 1-based line number.
func extractContext(fileLines []string, lineNum int) LineContext {
	if lineNum < 1 || lineNum > len(fileLines) {
		return LineContext{}
	}

	ctx := LineContext{
		Line: fileLines[lineNum-1],
	}

	// Extract lines above (up to ContextLines)
	for i := lineNum - ContextLines; i < lineNum; i++ {
		if i >= 1 {
			ctx.Above = append(ctx.Above, fileLines[i-1])
		}
	}

	// Extract lines below (up to ContextLines)
	for i := lineNum + 1; i <= lineNum+ContextLines; i++ {
		if i <= len(fileLines) {
			ctx.Below = append(ctx.Below, fileLines[i-1])
		}
	}

	return ctx
}

// SplitLines splits file content into lines, handling both \n and \r\n.
func SplitLines(content string) []string {
	// Normalize line endings
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	// Split and remove trailing empty line if present
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
