package aligner

import (
	"duckdiff/parser"
)

type LineType int

const (
	Unchanged LineType = iota
	Added
	Deleted
	Modified
)

type AlignedLine struct {
	OldLine    *string
	NewLine    *string
	LineType   LineType
	OldLineNum int
	NewLineNum int
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