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
				Old: Line{Num: oldNum, Content: line.Content, Type: Context},
				New: Line{Num: newNum, Content: line.Content, Type: Context},
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
				Old: Line{Num: 0, Content: "", Type: Empty},
				New: Line{Num: newNum, Content: line.Content, Type: Added},
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
			pair.Old = Line{Num: *oldNum, Content: removes[j].Content, Type: Removed}
			*oldNum++
		} else {
			pair.Old = Line{Num: 0, Content: "", Type: Empty}
		}

		if j < len(adds) {
			pair.New = Line{Num: *newNum, Content: adds[j].Content, Type: Added}
			*newNum++
		} else {
			pair.New = Line{Num: 0, Content: "", Type: Empty}
		}

		pairs = append(pairs, pair)
	}

	return pairs
}

// TransformFile converts a file diff into a FilePair.
func TransformFile(file diff.File) FilePair {
	fp := FilePair{
		OldPath:      file.OldPath,
		NewPath:      file.NewPath,
		Truncated:    file.Truncated,
		TotalAdded:   file.TotalAdded,
		TotalRemoved: file.TotalRemoved,
		IsRename:     file.IsRename,
		IsCopy:       file.IsCopy,
		Similarity:   file.Similarity,
		IsBinary:     file.IsBinary,
	}

	// Set per-side truncation flags based on which sides exist
	// If the diff was truncated, mark each side that exists as truncated
	if file.Truncated {
		if file.OldPath != "/dev/null" {
			fp.OldTruncated = true
		}
		if file.NewPath != "/dev/null" {
			fp.NewTruncated = true
		}
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

// SkeletonFilePair creates a FilePair with only path and stats (no Pairs).
// Used for lazy loading where the full diff is fetched on demand.
func SkeletonFilePair(path string, added, removed int) FilePair {
	isBinary := added < 0 || removed < 0
	if isBinary {
		added = 0
		removed = 0
	}
	return FilePair{
		OldPath:      "a/" + path,
		NewPath:      "b/" + path,
		TotalAdded:   added,
		TotalRemoved: removed,
		IsBinary:     isBinary,
		FoldLevel:    FoldFolded, // Start folded since no content yet
	}
}

// SkeletonFilePairNoStats creates a FilePair with only path (no stats or Pairs).
// Used for progressive loading where stats are fetched asynchronously.
func SkeletonFilePairNoStats(path string) FilePair {
	return FilePair{
		OldPath:      "a/" + path,
		NewPath:      "b/" + path,
		TotalAdded:   0,
		TotalRemoved: 0,
		FoldLevel:    FoldFolded, // Start folded since no content yet
	}
}
