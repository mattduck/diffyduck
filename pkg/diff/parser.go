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

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Start of a new file diff
		if diffHeaderRe.MatchString(line) {
			// Save the last hunk of the previous file before moving on
			if currentHunk != nil && currentFile != nil {
				currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
			}
			if currentFile != nil {
				diff.Files = append(diff.Files, *currentFile)
			}
			currentFile = &File{}
			currentHunk = nil
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
				diff.Files = append(diff.Files, *currentFile)
				currentFile = &File{}
			} else if currentFile == nil {
				currentFile = &File{}
			}
			currentFile.OldPath = m[1]
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
			switch line[0] {
			case ' ':
				currentHunk.Lines = append(currentHunk.Lines, Line{
					Type:    Context,
					Content: line[1:],
				})
			case '+':
				currentHunk.Lines = append(currentHunk.Lines, Line{
					Type:    Added,
					Content: line[1:],
				})
			case '-':
				currentHunk.Lines = append(currentHunk.Lines, Line{
					Type:    Removed,
					Content: line[1:],
				})
			}
		}
	}

	// Don't forget the last hunk and file
	if currentHunk != nil && currentFile != nil {
		currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
	}
	if currentFile != nil {
		diff.Files = append(diff.Files, *currentFile)
	}

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
