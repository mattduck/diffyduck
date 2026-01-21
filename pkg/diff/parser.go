package diff

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	diffHeaderRe = regexp.MustCompile(`^diff --git`)
	oldFileRe    = regexp.MustCompile(`^--- (.+)$`)
	newFileRe    = regexp.MustCompile(`^\+\+\+ (.+)$`)
	hunkHeaderRe = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)
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
			saveFile()
			currentFile = &File{}
			currentHunk = nil
			fileLineCount = 0
			continue
		}

		// Old file path
		if m := oldFileRe.FindStringSubmatch(line); m != nil {
			// Support standard unified diff (no "diff --git" header)
			// If we already have a file in progress, save it and start a new one
			if currentFile != nil && currentFile.OldPath != "" {
				// Save the last hunk of the previous file
				if currentHunk != nil {
					currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
					currentHunk = nil
				}
				saveFile()
				currentFile = &File{}
				fileLineCount = 0
			} else if currentFile == nil {
				currentFile = &File{}
				fileLineCount = 0
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
		if currentHunk != nil && len(line) > 0 {
			// Check if we've hit the line limit for this file
			if fileLineCount >= MaxLinesPerFile {
				if currentFile != nil {
					currentFile.Truncated = true
				}
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
