package tui

import (
	"github.com/user/diffyduck/pkg/sidebyside"
	"github.com/user/diffyduck/pkg/structure"
)

// SemanticContextThreshold is the maximum distance (in lines) from a hunk boundary
// to a scope boundary (function/class start) before we expand the context.
const SemanticContextThreshold = 20

// hunkBoundary represents the start and end indices within Pairs for a single hunk.
type hunkBoundary struct {
	startIdx int // index in Pairs where this hunk starts
	endIdx   int // index in Pairs where this hunk ends (exclusive)
}

// findHunkBoundaries identifies where hunks start and end in a file's Pairs.
// A hunk boundary is detected when line numbers jump by more than 1.
func findHunkBoundaries(pairs []sidebyside.LinePair) []hunkBoundary {
	if len(pairs) == 0 {
		return nil
	}

	var boundaries []hunkBoundary
	hunkStart := 0
	var prevOld, prevNew int

	for i, pair := range pairs {
		if i > 0 {
			// Check for gap in line numbers (hunk boundary)
			oldGap := prevOld > 0 && pair.Old.Num > 0 && pair.Old.Num > prevOld+1
			newGap := prevNew > 0 && pair.New.Num > 0 && pair.New.Num > prevNew+1
			if oldGap || newGap {
				// End the previous hunk, start a new one
				boundaries = append(boundaries, hunkBoundary{startIdx: hunkStart, endIdx: i})
				hunkStart = i
			}
		}
		if pair.Old.Num > 0 {
			prevOld = pair.Old.Num
		}
		if pair.New.Num > 0 {
			prevNew = pair.New.Num
		}
	}

	// Don't forget the last hunk
	boundaries = append(boundaries, hunkBoundary{startIdx: hunkStart, endIdx: len(pairs)})

	return boundaries
}

// getFirstNewLineNum returns the first non-zero new line number in a slice of pairs.
func getFirstNewLineNum(pairs []sidebyside.LinePair) int {
	for _, p := range pairs {
		if p.New.Num > 0 {
			return p.New.Num
		}
	}
	return 0
}

// getFirstOldLineNum returns the first non-zero old line number in a slice of pairs.
func getFirstOldLineNum(pairs []sidebyside.LinePair) int {
	for _, p := range pairs {
		if p.Old.Num > 0 {
			return p.Old.Num
		}
	}
	return 0
}

// expandSemanticContext expands hunk context to include nearby scope boundaries.
// It modifies fp.Pairs in place by prepending/appending context lines when a
// function or class boundary is within threshold lines of the hunk boundary.
//
// Requires full file content (fp.NewContent) to be available.
func expandSemanticContext(fp *sidebyside.FilePair, newStruct *structure.Map, threshold int) {
	if newStruct == nil || len(fp.NewContent) == 0 || len(fp.Pairs) == 0 {
		return
	}

	boundaries := findHunkBoundaries(fp.Pairs)
	if len(boundaries) == 0 {
		return
	}

	// Process hunks in reverse order so index modifications don't affect later hunks
	for i := len(boundaries) - 1; i >= 0; i-- {
		hunk := boundaries[i]
		hunkPairs := fp.Pairs[hunk.startIdx:hunk.endIdx]

		// Expand start of hunk
		firstNewLine := getFirstNewLineNum(hunkPairs)
		firstOldLine := getFirstOldLineNum(hunkPairs)
		if firstNewLine > 1 {
			entries := newStruct.AtLine(firstNewLine)
			if len(entries) > 0 {
				// Find the innermost function/method scope
				innermost := findInnermostFunction(entries)
				if innermost != nil && innermost.StartLine < firstNewLine {
					gap := firstNewLine - innermost.StartLine
					if gap <= threshold {
						// Derive old-side start from the offset between old and new
						oldStart := 0
						if firstOldLine > 0 {
							oldStart = firstOldLine - gap
						}
						newPairs := buildContextPairs(fp, innermost.StartLine, firstNewLine-1, oldStart)
						fp.Pairs = insertPairs(fp.Pairs, hunk.startIdx, newPairs)
						// Adjust boundary indices for later iterations
						for j := 0; j < i; j++ {
							boundaries[j].startIdx += len(newPairs)
							boundaries[j].endIdx += len(newPairs)
						}
					}
				}
			}
		}
	}
}

// findInnermostFunction returns the innermost function/method entry from a list
// of structure entries (ordered outermost to innermost). Returns nil if none found.
func findInnermostFunction(entries []structure.Entry) *structure.Entry {
	// Entries are ordered outermost to innermost, so scan from end
	for i := len(entries) - 1; i >= 0; i-- {
		e := &entries[i]
		if e.Kind == "func" || e.Kind == "def" || e.Kind == "method" {
			return e
		}
	}
	return nil
}

// buildContextPairs creates context LinePairs from full file content.
// startLine and endLine are 1-based inclusive line numbers in NewContent.
// oldStartLine is the corresponding old-side line number (0 if unknown).
func buildContextPairs(fp *sidebyside.FilePair, startLine, endLine, oldStartLine int) []sidebyside.LinePair {
	var pairs []sidebyside.LinePair

	oldNum := oldStartLine
	for lineNum := startLine; lineNum <= endLine; lineNum++ {
		newIdx := lineNum - 1
		if newIdx < 0 || newIdx >= len(fp.NewContent) {
			continue
		}

		pair := sidebyside.LinePair{
			Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
			New: sidebyside.Line{Num: lineNum, Content: fp.NewContent[newIdx], Type: sidebyside.Context},
		}

		if oldNum > 0 && len(fp.OldContent) > 0 {
			oldIdx := oldNum - 1
			if oldIdx >= 0 && oldIdx < len(fp.OldContent) {
				pair.Old = sidebyside.Line{
					Num:     oldNum,
					Content: fp.OldContent[oldIdx],
					Type:    sidebyside.Context,
				}
			}
			oldNum++
		}

		pairs = append(pairs, pair)
	}

	return pairs
}

// insertPairs inserts newPairs into pairs at the given index.
func insertPairs(pairs []sidebyside.LinePair, idx int, newPairs []sidebyside.LinePair) []sidebyside.LinePair {
	if len(newPairs) == 0 {
		return pairs
	}
	result := make([]sidebyside.LinePair, 0, len(pairs)+len(newPairs))
	result = append(result, pairs[:idx]...)
	result = append(result, newPairs...)
	result = append(result, pairs[idx:]...)
	return result
}
