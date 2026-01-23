package diff

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	diffHeaderRe   = regexp.MustCompile(`^diff --git`)
	oldFileRe      = regexp.MustCompile(`^--- (.+)$`)
	newFileRe      = regexp.MustCompile(`^\+\+\+ (.+)$`)
	hunkHeaderRe   = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)
	renameFromRe   = regexp.MustCompile(`^rename from (.+)$`)
	renameToRe     = regexp.MustCompile(`^rename to (.+)$`)
	copyFromRe     = regexp.MustCompile(`^copy from (.+)$`)
	copyToRe       = regexp.MustCompile(`^copy to (.+)$`)
	similarityRe   = regexp.MustCompile(`^similarity index (\d+)%`)
	diffGitPathsRe = regexp.MustCompile(`^diff --git a/(.+) b/(.+)$`)
)

// Parse parses a unified diff string into a Diff structure.
func Parse(input string) (*Diff, error) {
	if input == "" {
		return &Diff{}, nil
	}

	lines := strings.Split(input, "\n")
	diff := &Diff{}
	var currentFile *File
	var currentHunk *Hunk
	var fileLineCount int
	var inTruncatedGitDiff bool // true if we saw a diff --git but couldn't create a file due to limit

	// saveFile saves the current file to the diff, respecting MaxFiles limit.
	// Returns true if the file was saved, false if we've hit the limit.
	saveFile := func() bool {
		if currentFile == nil {
			return true
		}
		if len(diff.Files) >= MaxFiles {
			diff.TruncatedFileCount++
			return false
		}
		diff.Files = append(diff.Files, *currentFile)
		return true
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Start of a new file diff
		if diffHeaderRe.MatchString(line) {
			// Save the last hunk of the previous file before moving on
			if currentHunk != nil && currentFile != nil {
				currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
			}
			saved := saveFile()
			currentHunk = nil
			fileLineCount = 0
			inTruncatedGitDiff = false

			if !saved || len(diff.Files) >= MaxFiles {
				// We've hit the file limit, don't create new files
				// Mark that we're in a truncated git diff block so --- doesn't double-count
				if len(diff.Files) >= MaxFiles && saved {
					// saveFile() returned true (nothing to save) but we're at limit
					// Count this new file as truncated
					diff.TruncatedFileCount++
				}
				currentFile = nil
				inTruncatedGitDiff = true
				continue
			}

			currentFile = &File{Similarity: -1} // -1 means not present

			// Extract fallback filenames from "diff --git a/X b/Y" header
			// These will be overridden by --- and +++ lines if present
			if m := diffGitPathsRe.FindStringSubmatch(line); m != nil {
				currentFile.OldPath = "a/" + m[1]
				currentFile.NewPath = "b/" + m[2]
			}
			continue
		}

		// Git metadata lines (between diff header and ---/+++ lines)
		if currentFile != nil {
			// New file mode - mark OldPath as /dev/null for proper status display
			if strings.HasPrefix(line, "new file mode") {
				currentFile.OldPath = "/dev/null"
				continue
			}
			// Deleted file mode - mark NewPath as /dev/null for proper status display
			if strings.HasPrefix(line, "deleted file mode") {
				currentFile.NewPath = "/dev/null"
				continue
			}
			// Rename metadata
			if m := renameFromRe.FindStringSubmatch(line); m != nil {
				currentFile.IsRename = true
				currentFile.OldPath = "a/" + m[1]
				continue
			}
			if m := renameToRe.FindStringSubmatch(line); m != nil {
				currentFile.IsRename = true
				currentFile.NewPath = "b/" + m[1]
				continue
			}
			// Copy metadata
			if m := copyFromRe.FindStringSubmatch(line); m != nil {
				currentFile.IsCopy = true
				currentFile.OldPath = "a/" + m[1]
				continue
			}
			if m := copyToRe.FindStringSubmatch(line); m != nil {
				currentFile.IsCopy = true
				currentFile.NewPath = "b/" + m[1]
				continue
			}
			// Similarity index
			if m := similarityRe.FindStringSubmatch(line); m != nil {
				currentFile.Similarity = mustAtoi(m[1])
				continue
			}
		}

		// Old file path (--- line)
		if m := oldFileRe.FindStringSubmatch(line); m != nil {
			// If we're in a truncated git diff block, this --- is for the same file
			// that was already counted as truncated in the diff --git handler
			if inTruncatedGitDiff {
				inTruncatedGitDiff = false
				continue
			}

			// Support standard unified diff (no "diff --git" header)
			// Only create a new file if we have an in-progress file with content
			// (i.e., we've moved past the header into hunks)
			// Check both saved hunks and the current unsaved hunk
			hasContent := currentFile != nil && (len(currentFile.Hunks) > 0 || currentHunk != nil)
			if hasContent {
				// Save the last hunk of the previous file
				if currentHunk != nil {
					currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
					currentHunk = nil
				}
				saved := saveFile()
				if !saved {
					// We've hit the file limit, just count remaining files as truncated
					currentFile = nil
					continue
				}
				currentFile = &File{Similarity: -1}
				fileLineCount = 0
			} else if currentFile == nil {
				// Check if we've already hit the limit (this can happen after
				// we stopped creating files due to truncation)
				if len(diff.Files) >= MaxFiles {
					diff.TruncatedFileCount++
					continue
				}
				currentFile = &File{Similarity: -1}
				fileLineCount = 0
			} else {
				// currentFile exists but has no content - this means either:
				// 1. Git diff: created from diff --git, this --- confirms the path
				// 2. Standard unified diff: previous file's content was skipped (at limit)
				//    and this --- is for a NEW file
				//
				// In case 1, the path matches and we just update OldPath.
				// In case 2, we need to count the current file AND the new file.
				if len(diff.Files) >= MaxFiles {
					// Count the current file that couldn't get content
					diff.TruncatedFileCount++
					// For standard unified diff, also count the new file we can't start
					// (In git diff format, inTruncatedGitDiff would have been set)
					if !inTruncatedGitDiff && m[1] != currentFile.OldPath {
						// This is a different file in standard unified diff format
						diff.TruncatedFileCount++
					}
					currentFile = nil // Prevent saveFile() from counting again
					continue
				}
				// Don't create a new file, just update OldPath below
			}
			currentFile.OldPath = m[1]
			continue
		}

		// Skip processing content if we've already hit the file limit
		if len(diff.Files) >= MaxFiles {
			continue
		}

		// New file path
		if m := newFileRe.FindStringSubmatch(line); m != nil {
			if currentFile != nil {
				currentFile.NewPath = m[1]
			}
			continue
		}

		// Hunk header
		if m := hunkHeaderRe.FindStringSubmatch(line); m != nil {
			if currentHunk != nil && currentFile != nil {
				currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
			}
			currentHunk = &Hunk{
				OldStart: mustAtoi(m[1]),
				OldCount: atoiOrDefault(m[2], 1),
				NewStart: mustAtoi(m[3]),
				NewCount: atoiOrDefault(m[4], 1),
			}
			continue
		}

		// Skip "\ No newline at end of file" markers
		if strings.HasPrefix(line, `\`) {
			continue
		}

		// Diff content lines
		if currentHunk != nil && currentFile != nil && len(line) > 0 {
			// Always count added/removed lines for accurate stats
			switch line[0] {
			case '+':
				currentFile.TotalAdded++
			case '-':
				currentFile.TotalRemoved++
			}

			// Check if we've hit the line limit for this file
			if fileLineCount >= MaxLinesPerFile {
				currentFile.Truncated = true
				continue
			}

			switch line[0] {
			case ' ':
				currentHunk.Lines = append(currentHunk.Lines, Line{
					Type:    Context,
					Content: truncateLine(line[1:]),
				})
				fileLineCount++
			case '+':
				currentHunk.Lines = append(currentHunk.Lines, Line{
					Type:    Added,
					Content: truncateLine(line[1:]),
				})
				fileLineCount++
			case '-':
				currentHunk.Lines = append(currentHunk.Lines, Line{
					Type:    Removed,
					Content: truncateLine(line[1:]),
				})
				fileLineCount++
			}
		}
	}

	// Don't forget the last hunk and file
	if currentHunk != nil && currentFile != nil {
		currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
	}
	saveFile()

	return diff, nil
}

func mustAtoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func atoiOrDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// truncateLine truncates a line if it exceeds MaxLineLength, appending the truncation suffix.
func truncateLine(content string) string {
	if len(content) <= MaxLineLength {
		return content
	}
	// Leave room for the truncation text
	cutoff := MaxLineLength - len(LineTruncationText)
	if cutoff < 0 {
		cutoff = 0
	}
	return content[:cutoff] + LineTruncationText
}
