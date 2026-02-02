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

// getLastNewLineNum returns the last non-zero new line number in a slice of pairs.
func getLastNewLineNum(pairs []sidebyside.LinePair) int {
	for i := len(pairs) - 1; i >= 0; i-- {
		if pairs[i].New.Num > 0 {
			return pairs[i].New.Num
		}
	}
	return 0
}

// getLastOldLineNum returns the last non-zero old line number in a slice of pairs.
func getLastOldLineNum(pairs []sidebyside.LinePair) int {
	for i := len(pairs) - 1; i >= 0; i-- {
		if pairs[i].Old.Num > 0 {
			return pairs[i].Old.Num
		}
	}
	return 0
}

// ContextExpandAmount is the number of context lines to add per Enter press.
const ContextExpandAmount = 15

// expandContextDown appends context lines after a hunk (expanding downward into the gap).
// hunkIdx is the index into boundaries of the hunk to expand after.
// Returns the number of lines inserted.
func expandContextDown(fp *sidebyside.FilePair, boundaries []hunkBoundary, hunkIdx int) int {
	if hunkIdx < 0 || hunkIdx >= len(boundaries) {
		return 0
	}

	hunk := boundaries[hunkIdx]
	hunkPairs := fp.Pairs[hunk.startIdx:hunk.endIdx]

	lastNew := getLastNewLineNum(hunkPairs)
	lastOld := getLastOldLineNum(hunkPairs)
	if lastNew == 0 {
		return 0
	}

	// Start of new context: line after end of this hunk
	newStart := lastNew + 1
	newEnd := lastNew + ContextExpandAmount

	// Clamp to file boundary
	if len(fp.NewContent) > 0 && newEnd > len(fp.NewContent) {
		newEnd = len(fp.NewContent)
	}

	// Clamp to next hunk boundary (don't overlap)
	if hunkIdx+1 < len(boundaries) {
		nextHunk := boundaries[hunkIdx+1]
		nextHunkPairs := fp.Pairs[nextHunk.startIdx:nextHunk.endIdx]
		firstNextNew := getFirstNewLineNum(nextHunkPairs)
		if firstNextNew > 0 && newEnd >= firstNextNew {
			newEnd = firstNextNew - 1
		}
	}

	if newStart > newEnd {
		return 0
	}

	// Derive old-side start from offset at end of hunk
	oldStart := 0
	if lastOld > 0 {
		oldStart = lastOld + 1
	}

	newPairs := buildContextPairs(fp, newStart, newEnd, oldStart)
	if len(newPairs) == 0 {
		return 0
	}

	// Insert after the last pair of this hunk
	fp.Pairs = insertPairs(fp.Pairs, hunk.endIdx, newPairs)
	return len(newPairs)
}

// expandContextUp prepends context lines before a hunk (expanding upward into the gap).
// hunkIdx is the index into boundaries of the hunk to expand before.
// Returns the number of lines inserted.
func expandContextUp(fp *sidebyside.FilePair, boundaries []hunkBoundary, hunkIdx int) int {
	if hunkIdx < 0 || hunkIdx >= len(boundaries) {
		return 0
	}

	hunk := boundaries[hunkIdx]
	hunkPairs := fp.Pairs[hunk.startIdx:hunk.endIdx]

	firstNew := getFirstNewLineNum(hunkPairs)
	firstOld := getFirstOldLineNum(hunkPairs)
	if firstNew <= 1 {
		return 0 // already at file start
	}

	// End of new context: line before start of this hunk
	newEnd := firstNew - 1
	newStart := firstNew - ContextExpandAmount

	// Clamp to file start
	if newStart < 1 {
		newStart = 1
	}

	// Clamp to previous hunk boundary (don't overlap)
	if hunkIdx > 0 {
		prevHunk := boundaries[hunkIdx-1]
		prevHunkPairs := fp.Pairs[prevHunk.startIdx:prevHunk.endIdx]
		lastPrevNew := getLastNewLineNum(prevHunkPairs)
		if lastPrevNew > 0 && newStart <= lastPrevNew {
			newStart = lastPrevNew + 1
		}
	}

	if newStart > newEnd {
		return 0
	}

	// Derive old-side start from offset at start of hunk
	oldStart := 0
	if firstOld > 0 {
		gap := firstNew - newStart
		oldStart = firstOld - gap
		if oldStart < 1 {
			oldStart = 1
		}
	}

	newPairs := buildContextPairs(fp, newStart, newEnd, oldStart)
	if len(newPairs) == 0 {
		return 0
	}

	// Insert before the first pair of this hunk
	fp.Pairs = insertPairs(fp.Pairs, hunk.startIdx, newPairs)
	return len(newPairs)
}

// expandContextToSignature prepends context lines before a hunk to include a
// scope boundary (function/class signature). Expands up to signatureLine - 2
// (2 lines of padding above the signature).
// hunkIdx is the index into boundaries of the hunk to expand before.
// Returns the number of lines inserted.
func expandContextToSignature(fp *sidebyside.FilePair, boundaries []hunkBoundary, hunkIdx int, signatureLine int) int {
	if hunkIdx < 0 || hunkIdx >= len(boundaries) {
		return 0
	}

	hunk := boundaries[hunkIdx]
	hunkPairs := fp.Pairs[hunk.startIdx:hunk.endIdx]

	firstNew := getFirstNewLineNum(hunkPairs)
	firstOld := getFirstOldLineNum(hunkPairs)
	if firstNew <= 1 {
		return 0
	}

	// Target: signature line minus 2 lines of padding
	newStart := signatureLine - 2
	newEnd := firstNew - 1

	// Clamp to file start
	if newStart < 1 {
		newStart = 1
	}

	// Clamp to previous hunk boundary
	if hunkIdx > 0 {
		prevHunk := boundaries[hunkIdx-1]
		prevHunkPairs := fp.Pairs[prevHunk.startIdx:prevHunk.endIdx]
		lastPrevNew := getLastNewLineNum(prevHunkPairs)
		if lastPrevNew > 0 && newStart <= lastPrevNew {
			newStart = lastPrevNew + 1
		}
	}

	if newStart > newEnd {
		return 0
	}

	// Derive old-side start
	oldStart := 0
	if firstOld > 0 {
		gap := firstNew - newStart
		oldStart = firstOld - gap
		if oldStart < 1 {
			oldStart = 1
		}
	}

	newPairs := buildContextPairs(fp, newStart, newEnd, oldStart)
	if len(newPairs) == 0 {
		return 0
	}

	fp.Pairs = insertPairs(fp.Pairs, hunk.startIdx, newPairs)
	return len(newPairs)
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

	// Process hunks in reverse order so insertions don't affect later hunks.
	// Since we insert BEFORE hunk i, only pairs at index >= hunk.startIdx shift.
	// Boundaries for hunks j < i are at earlier indices and are unaffected.
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
				innermost := findInnermostEntry(entries)
				if innermost != nil && innermost.StartLine < firstNewLine {
					gap := firstNewLine - innermost.StartLine
					if gap <= threshold {
						// Derive old-side start from the offset between old and new
						oldStart := 0
						if firstOldLine > 0 {
							oldStart = firstOldLine - gap
						}

						// Clamp to previous hunk boundary (don't overlap)
						startLine := innermost.StartLine
						if i > 0 {
							prevHunk := boundaries[i-1]
							prevHunkPairs := fp.Pairs[prevHunk.startIdx:prevHunk.endIdx]
							lastPrevNew := getLastNewLineNum(prevHunkPairs)
							if lastPrevNew > 0 && startLine <= lastPrevNew {
								startLine = lastPrevNew + 1
								// Recalculate old start for the clamped range
								if firstOldLine > 0 {
									clampedGap := firstNewLine - startLine
									oldStart = firstOldLine - clampedGap
								}
							}
						}
						if startLine >= firstNewLine {
							continue
						}

						newPairs := buildContextPairs(fp, startLine, firstNewLine-1, oldStart)
						fp.Pairs = insertPairs(fp.Pairs, hunk.startIdx, newPairs)
					}
				}
			}
		}
	}
}

// findInnermostEntry returns the innermost structure entry from a list
// of entries (ordered outermost to innermost). Returns nil if empty.
func findInnermostEntry(entries []structure.Entry) *structure.Entry {
	if len(entries) == 0 {
		return nil
	}
	return &entries[len(entries)-1]
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
