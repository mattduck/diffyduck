package aligner

import (
	"regexp"

	"diffyduck/parser"
	"diffyduck/types"
	"github.com/sergi/go-diff/diffmatchpatch"
)

type LineType int

const (
	Unchanged LineType = iota
	Added
	Deleted
	Modified
)

type WordDiff struct {
	OldSegments []types.DiffSegment
	NewSegments []types.DiffSegment
}

type AlignedLine struct {
	OldLine    *string
	NewLine    *string
	LineType   LineType
	OldLineNum int
	NewLineNum int
	WordDiff   *WordDiff
}

type DiffAligner struct{}

func NewDiffAligner() *DiffAligner {
	return &DiffAligner{}
}

func (a *DiffAligner) AlignFile(oldLines, newLines []string, hunks []parser.Hunk) []AlignedLine {
	var result []AlignedLine

	oldPos := 1
	newPos := 1

	for _, hunk := range hunks {
		result = append(result, a.addUnchangedLines(oldLines, newLines, oldPos, newPos, hunk.OldStart, hunk.NewStart)...)

		oldPos = hunk.OldStart
		newPos = hunk.NewStart

		alignedHunk, newOldPos, newNewPos := a.processHunk(hunk, oldLines, newLines, oldPos, newPos)
		result = append(result, alignedHunk...)

		oldPos = newOldPos
		newPos = newNewPos
	}

	result = append(result, a.addUnchangedLines(oldLines, newLines, oldPos, newPos, len(oldLines)+1, len(newLines)+1)...)

	// Post-process to detect modifications
	result = a.detectModifications(result)

	return result
}

func (a *DiffAligner) addUnchangedLines(oldLines, newLines []string, oldStart, newStart, oldEnd, newEnd int) []AlignedLine {
	var result []AlignedLine

	oldIdx := oldStart - 1
	newIdx := newStart - 1

	for oldIdx < oldEnd-1 && newIdx < newEnd-1 && oldIdx < len(oldLines) && newIdx < len(newLines) {
		oldLine := oldLines[oldIdx]
		newLine := newLines[newIdx]

		result = append(result, AlignedLine{
			OldLine:    &oldLine,
			NewLine:    &newLine,
			LineType:   Unchanged,
			OldLineNum: oldIdx + 1,
			NewLineNum: newIdx + 1,
		})

		oldIdx++
		newIdx++
	}

	return result
}

func (a *DiffAligner) processHunk(hunk parser.Hunk, oldLines, newLines []string, oldPos, newPos int) ([]AlignedLine, int, int) {
	var result []AlignedLine

	currentOldPos := oldPos
	currentNewPos := newPos

	for _, line := range hunk.Lines {
		if len(line) == 0 {
			continue
		}

		prefix := line[0]
		content := line[1:]

		switch prefix {
		case ' ':
			result = append(result, AlignedLine{
				OldLine:    &content,
				NewLine:    &content,
				LineType:   Unchanged,
				OldLineNum: currentOldPos,
				NewLineNum: currentNewPos,
			})
			currentOldPos++
			currentNewPos++

		case '-':
			result = append(result, AlignedLine{
				OldLine:    &content,
				NewLine:    nil,
				LineType:   Deleted,
				OldLineNum: currentOldPos,
				NewLineNum: 0,
			})
			currentOldPos++

		case '+':
			result = append(result, AlignedLine{
				OldLine:    nil,
				NewLine:    &content,
				LineType:   Added,
				OldLineNum: 0,
				NewLineNum: currentNewPos,
			})
			currentNewPos++
		}
	}

	return result, currentOldPos, currentNewPos
}

func (a *DiffAligner) detectModifications(lines []AlignedLine) []AlignedLine {
	var result []AlignedLine
	i := 0

	for i < len(lines) {
		if i < len(lines) && lines[i].LineType == Deleted {
			// Collect consecutive deletions
			deletions := []AlignedLine{}
			for i < len(lines) && lines[i].LineType == Deleted {
				deletions = append(deletions, lines[i])
				i++
			}

			// Collect consecutive additions immediately following
			additions := []AlignedLine{}
			for i < len(lines) && lines[i].LineType == Added {
				additions = append(additions, lines[i])
				i++
			}

			// Pair them up as modifications
			pairs := len(deletions)
			if len(additions) < pairs {
				pairs = len(additions)
			}

			for p := 0; p < pairs; p++ {
				wordDiff := a.computeWordDiff(*deletions[p].OldLine, *additions[p].NewLine)
				result = append(result, AlignedLine{
					OldLine:    deletions[p].OldLine,
					NewLine:    additions[p].NewLine,
					LineType:   Modified,
					OldLineNum: deletions[p].OldLineNum,
					NewLineNum: additions[p].NewLineNum,
					WordDiff:   wordDiff,
				})
			}

			// Add remaining unpaired deletions
			for p := pairs; p < len(deletions); p++ {
				result = append(result, deletions[p])
			}

			// Add remaining unpaired additions
			for p := pairs; p < len(additions); p++ {
				result = append(result, additions[p])
			}
		} else {
			// Not a deletion, just copy the line
			result = append(result, lines[i])
			i++
		}
	}

	return result
}

func (a *DiffAligner) computeWordDiff(oldLine, newLine string) *WordDiff {
	// Handle identical strings quickly
	if oldLine == newLine {
		if oldLine == "" {
			return &WordDiff{
				OldSegments: []types.DiffSegment{},
				NewSegments: []types.DiffSegment{},
			}
		}
		// Both lines are identical and non-empty
		segments := []types.DiffSegment{{
			Text: oldLine,
			Type: diffmatchpatch.DiffEqual,
		}}
		return &WordDiff{
			OldSegments: segments,
			NewSegments: segments,
		}
	}

	// Tokenize by words/whitespace for better diffs
	oldTokens := a.tokenize(oldLine)
	newTokens := a.tokenize(newLine)

	// Compute diff on word token arrays directly
	diffs := a.computeTokenDiff(oldTokens, newTokens)

	// Build segment lists for old and new versions
	oldSegments := a.buildSegments(diffs, true)
	newSegments := a.buildSegments(diffs, false)

	return &WordDiff{
		OldSegments: oldSegments,
		NewSegments: newSegments,
	}
}

func (a *DiffAligner) tokenize(text string) []string {
	// Handle empty string case
	if text == "" {
		return []string{}
	}

	// Split on word boundaries: capture words and whitespace separately
	re := regexp.MustCompile(`(\w+|\s+|[^\w\s]+)`)
	tokens := re.FindAllString(text, -1)

	// Filter out any empty tokens (shouldn't happen with this regex, but defensive)
	if tokens == nil {
		return []string{}
	}

	// Remove any empty strings that might slip through
	filtered := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token != "" {
			filtered = append(filtered, token)
		}
	}

	return filtered
}

func (a *DiffAligner) buildSegments(diffs []diffmatchpatch.Diff, isOld bool) []types.DiffSegment {
	var segments []types.DiffSegment

	for _, diff := range diffs {
		switch diff.Type {
		case diffmatchpatch.DiffEqual:
			segments = append(segments, types.DiffSegment{
				Text: diff.Text,
				Type: diffmatchpatch.DiffEqual,
			})
		case diffmatchpatch.DiffDelete:
			if isOld {
				// Include deleted text in old version
				segments = append(segments, types.DiffSegment{
					Text: diff.Text,
					Type: diffmatchpatch.DiffDelete,
				})
			}
			// Don't include deleted text in new version
		case diffmatchpatch.DiffInsert:
			if !isOld {
				// Include inserted text in new version
				segments = append(segments, types.DiffSegment{
					Text: diff.Text,
					Type: diffmatchpatch.DiffInsert,
				})
			}
			// Don't include inserted text in old version
		}
	}

	return segments
}

func (a *DiffAligner) computeTokenDiff(oldTokens, newTokens []string) []diffmatchpatch.Diff {
	// Implement a simple word-level LCS algorithm
	return a.wordLevelDiff(oldTokens, newTokens)
}

func (a *DiffAligner) wordLevelDiff(oldTokens, newTokens []string) []diffmatchpatch.Diff {
	// Handle trivial cases first
	if len(oldTokens) == 0 && len(newTokens) == 0 {
		return []diffmatchpatch.Diff{}
	}
	if len(oldTokens) == 0 {
		return a.allInsertions(newTokens)
	}
	if len(newTokens) == 0 {
		return a.allDeletions(oldTokens)
	}

	// Simple word-level diff using dynamic programming
	m, n := len(oldTokens), len(newTokens)

	// Create LCS table
	lcs := make([][]int, m+1)
	for i := range lcs {
		lcs[i] = make([]int, n+1)
	}

	// Fill LCS table
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldTokens[i-1] == newTokens[j-1] {
				lcs[i][j] = lcs[i-1][j-1] + 1
			} else {
				if lcs[i-1][j] > lcs[i][j-1] {
					lcs[i][j] = lcs[i-1][j]
				} else {
					lcs[i][j] = lcs[i][j-1]
				}
			}
		}
	}

	// Trace back to build diff sequence (in reverse, then reverse the result)
	diffs := make([]diffmatchpatch.Diff, 0, m+n) // Pre-allocate with worst-case capacity
	i, j := m, n

	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldTokens[i-1] == newTokens[j-1] {
			// Equal tokens
			diffs = append(diffs, diffmatchpatch.Diff{
				Type: diffmatchpatch.DiffEqual,
				Text: oldTokens[i-1],
			})
			i--
			j--
		} else if i > 0 && (j == 0 || lcs[i-1][j] >= lcs[i][j-1]) {
			// Deletion
			diffs = append(diffs, diffmatchpatch.Diff{
				Type: diffmatchpatch.DiffDelete,
				Text: oldTokens[i-1],
			})
			i--
		} else {
			// Insertion
			diffs = append(diffs, diffmatchpatch.Diff{
				Type: diffmatchpatch.DiffInsert,
				Text: newTokens[j-1],
			})
			j--
		}
	}

	// Reverse the diffs since we built them backwards
	for i := 0; i < len(diffs)/2; i++ {
		diffs[i], diffs[len(diffs)-1-i] = diffs[len(diffs)-1-i], diffs[i]
	}

	return diffs
}

func (a *DiffAligner) allInsertions(tokens []string) []diffmatchpatch.Diff {
	if len(tokens) == 0 {
		return []diffmatchpatch.Diff{}
	}

	diffs := make([]diffmatchpatch.Diff, 0, len(tokens))
	for _, token := range tokens {
		if token != "" { // Skip empty tokens
			diffs = append(diffs, diffmatchpatch.Diff{
				Type: diffmatchpatch.DiffInsert,
				Text: token,
			})
		}
	}
	return diffs
}

func (a *DiffAligner) allDeletions(tokens []string) []diffmatchpatch.Diff {
	if len(tokens) == 0 {
		return []diffmatchpatch.Diff{}
	}

	diffs := make([]diffmatchpatch.Diff, 0, len(tokens))
	for _, token := range tokens {
		if token != "" { // Skip empty tokens
			diffs = append(diffs, diffmatchpatch.Diff{
				Type: diffmatchpatch.DiffDelete,
				Text: token,
			})
		}
	}
	return diffs
}
