package display

import (
	"fmt"
	"strings"

	"duckdiff/aligner"
	"duckdiff/git"
	"duckdiff/parser"
)

type Renderer struct {
	retriever *git.FileRetriever
	aligner   *aligner.DiffAligner
}

func NewRenderer() *Renderer {
	return &Renderer{
		retriever: git.NewFileRetriever(),
		aligner:   aligner.NewDiffAligner(),
	}
}

func (r *Renderer) RenderFileDiffs(fileDiffs []parser.FileDiff) error {
	for i, fileDiff := range fileDiffs {
		if i > 0 {
			fmt.Println()
		}
		
		err := r.renderSingleFile(fileDiff)
		if err != nil {
			return fmt.Errorf("error rendering file %s: %v", fileDiff.NewPath, err)
		}
	}
	return nil
}

func (r *Renderer) renderSingleFile(fileDiff parser.FileDiff) error {
	fmt.Printf("=== %s ===\n", fileDiff.NewPath)
	
	oldLines, err := r.retriever.GetOldFileContent(fileDiff.OldPath)
	if err != nil {
		return err
	}
	
	newLines, err := r.retriever.GetNewFileContent(fileDiff.NewPath)
	if err != nil {
		return err
	}
	
	alignedLines := r.aligner.AlignFile(oldLines, newLines, fileDiff.Hunks)
	r.renderAlignedLines(alignedLines)
	return nil
}

func (r *Renderer) renderAlignedLines(alignedLines []aligner.AlignedLine) {
	const leftWidth = 60
	
	fmt.Printf("%-*s | %s\n", leftWidth, "", "")
	fmt.Printf("%s-+-%s\n", strings.Repeat("-", leftWidth), strings.Repeat("-", leftWidth))
	
	for _, line := range alignedLines {
		var oldDisplay, newDisplay string
		
		if line.OldLine != nil {
			oldContent := *line.OldLine
			if len(oldContent) > leftWidth-2 {
				oldContent = oldContent[:leftWidth-5] + "..."
			}
			switch line.LineType {
			case aligner.Deleted:
				oldDisplay = fmt.Sprintf("-%s", oldContent)
			default:
				oldDisplay = fmt.Sprintf(" %s", oldContent)
			}
		} else {
			oldDisplay = ""
		}
		
		if line.NewLine != nil {
			newContent := *line.NewLine
			if len(newContent) > leftWidth-2 {
				newContent = newContent[:leftWidth-5] + "..."
			}
			switch line.LineType {
			case aligner.Added:
				newDisplay = fmt.Sprintf("+%s", newContent)
			default:
				newDisplay = fmt.Sprintf(" %s", newContent)
			}
		} else {
			newDisplay = ""
		}
		
		fmt.Printf("%-*s | %s\n", leftWidth, oldDisplay, newDisplay)
	}
}