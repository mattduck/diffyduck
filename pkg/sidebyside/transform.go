package sidebyside

import "github.com/user/diffyduck/pkg/diff"

// TransformHunk converts a diff hunk into side-by-side line pairs.
// It aligns removed and added lines when they appear consecutively (modifications).
func TransformHunk(hunk diff.Hunk) []LinePair {
	var pairs []LinePair
	oldNum := hunk.OldStart
	newNum := hunk.NewStart

	i := 0
	for i < len(hunk.Lines) {
		line := hunk.Lines[i]

		switch line.Type {
		case diff.Context:
			pairs = append(pairs, LinePair{
				Left:  Line{Num: oldNum, Content: line.Content, Type: Context},
				Right: Line{Num: newNum, Content: line.Content, Type: Context},
			})
			oldNum++
			newNum++
			i++

		case diff.Removed:
			// Collect consecutive removes
			var removes []diff.Line
			for i < len(hunk.Lines) && hunk.Lines[i].Type == diff.Removed {
				removes = append(removes, hunk.Lines[i])
				i++
			}

			// Collect consecutive adds that follow
			var adds []diff.Line
			for i < len(hunk.Lines) && hunk.Lines[i].Type == diff.Added {
				adds = append(adds, hunk.Lines[i])
				i++
			}

			// Pair them up
			pairs = append(pairs, alignChanges(removes, adds, &oldNum, &newNum)...)

		case diff.Added:
			// Added without preceding remove
			pairs = append(pairs, LinePair{
				Left:  Line{Num: 0, Content: "", Type: Empty},
				Right: Line{Num: newNum, Content: line.Content, Type: Added},
			})
			newNum++
			i++
		}
	}

	return pairs
}

// alignChanges pairs up removed and added lines, handling mismatched counts.
func alignChanges(removes, adds []diff.Line, oldNum, newNum *int) []LinePair {
	var pairs []LinePair

	maxLen := len(removes)
	if len(adds) > maxLen {
		maxLen = len(adds)
	}

	for j := 0; j < maxLen; j++ {
		pair := LinePair{}

		if j < len(removes) {
			pair.Left = Line{Num: *oldNum, Content: removes[j].Content, Type: Removed}
			*oldNum++
		} else {
			pair.Left = Line{Num: 0, Content: "", Type: Empty}
		}

		if j < len(adds) {
			pair.Right = Line{Num: *newNum, Content: adds[j].Content, Type: Added}
			*newNum++
		} else {
			pair.Right = Line{Num: 0, Content: "", Type: Empty}
		}

		pairs = append(pairs, pair)
	}

	return pairs
}

// TransformFile converts a file diff into a FilePair.
func TransformFile(file diff.File) FilePair {
	fp := FilePair{
		OldPath:   file.OldPath,
		NewPath:   file.NewPath,
		Truncated: file.Truncated,
	}

	for _, hunk := range file.Hunks {
		fp.Pairs = append(fp.Pairs, TransformHunk(hunk)...)
	}

	return fp
}

// TransformDiff converts a complete diff into a slice of FilePairs.
// Returns the file pairs and the count of truncated files (files omitted due to limit).
func TransformDiff(d *diff.Diff) ([]FilePair, int) {
	var fps []FilePair
	for _, file := range d.Files {
		fps = append(fps, TransformFile(file))
	}
	return fps, d.TruncatedFileCount
}
