package parser

import (
	"regexp"
	"strconv"
	"strings"
)

type FileDiff struct {
	OldPath string
	NewPath string
	Hunks   []Hunk
}

type Hunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []string
}

type DiffParser struct {
	fileHeaderRe *regexp.Regexp
	hunkHeaderRe *regexp.Regexp
	oldFileRe    *regexp.Regexp
	newFileRe    *regexp.Regexp
}

func NewDiffParser() *DiffParser {
	return &DiffParser{
		fileHeaderRe: regexp.MustCompile(`^diff --git a/(.*) b/(.*)$`),
		hunkHeaderRe: regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`),
		oldFileRe:    regexp.MustCompile(`^--- (.*)$`),
		newFileRe:    regexp.MustCompile(`^\+\+\+ (.*)$`),
	}
}

func (p *DiffParser) Parse(diffContent string) ([]FileDiff, error) {
	lines := strings.Split(diffContent, "\n")
	var fileDiffs []FileDiff
	var currentFile *FileDiff
	var currentHunk *Hunk

	for _, line := range lines {
		if matches := p.fileHeaderRe.FindStringSubmatch(line); matches != nil {
			if currentFile != nil {
				if currentHunk != nil {
					currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
				}
				fileDiffs = append(fileDiffs, *currentFile)
			}

			currentFile = &FileDiff{
				OldPath: matches[1],
				NewPath: matches[2],
				Hunks:   []Hunk{},
			}
			currentHunk = nil
		} else if matches := p.oldFileRe.FindStringSubmatch(line); matches != nil && currentFile != nil {
			// Update old path from --- line (handles /dev/null for new files)
			currentFile.OldPath = matches[1]
		} else if matches := p.newFileRe.FindStringSubmatch(line); matches != nil && currentFile != nil {
			// Update new path from +++ line (handles /dev/null for deleted files) 
			newPath := matches[1]
			if strings.HasPrefix(newPath, "b/") {
				newPath = newPath[2:] // Remove "b/" prefix
			}
			currentFile.NewPath = newPath
		} else if matches := p.hunkHeaderRe.FindStringSubmatch(line); matches != nil {
			if currentHunk != nil && currentFile != nil {
				currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
			}

			oldStart, _ := strconv.Atoi(matches[1])
			oldCount := 1
			if matches[2] != "" {
				oldCount, _ = strconv.Atoi(matches[2])
			}

			newStart, _ := strconv.Atoi(matches[3])
			newCount := 1
			if matches[4] != "" {
				newCount, _ = strconv.Atoi(matches[4])
			}

			currentHunk = &Hunk{
				OldStart: oldStart,
				OldCount: oldCount,
				NewStart: newStart,
				NewCount: newCount,
				Lines:    []string{},
			}
		} else if currentHunk != nil {
			if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
				currentHunk.Lines = append(currentHunk.Lines, line)
			}
		}
	}

	if currentFile != nil {
		if currentHunk != nil {
			currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
		}
		fileDiffs = append(fileDiffs, *currentFile)
	}

	return fileDiffs, nil
}