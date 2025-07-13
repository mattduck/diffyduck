package display

import (
	"fmt"
	"strings"

	"duckdiff/git"
	"duckdiff/parser"
)

type Renderer struct {
	retriever *git.FileRetriever
}

func NewRenderer() *Renderer {
	return &Renderer{
		retriever: git.NewFileRetriever(),
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
	
	r.renderSideBySide(oldLines, newLines)
	return nil
}

func (r *Renderer) renderSideBySide(oldLines, newLines []string) {
	maxLines := len(oldLines)
	if len(newLines) > maxLines {
		maxLines = len(newLines)
	}
	
	const leftWidth = 60
	
	fmt.Printf("%-*s | %s\n", leftWidth, "OLD", "NEW")
	fmt.Printf("%s-+-%s\n", strings.Repeat("-", leftWidth), strings.Repeat("-", leftWidth))
	
	for i := 0; i < maxLines; i++ {
		var oldLine, newLine string
		
		if i < len(oldLines) {
			oldLine = oldLines[i]
		} else {
			oldLine = ""
		}
		
		if i < len(newLines) {
			newLine = newLines[i]
		} else {
			newLine = ""
		}
		
		if len(oldLine) > leftWidth-1 {
			oldLine = oldLine[:leftWidth-4] + "..."
		}
		if len(newLine) > leftWidth-1 {
			newLine = newLine[:leftWidth-4] + "..."
		}
		
		fmt.Printf("%-*s | %s\n", leftWidth, oldLine, newLine)
	}
}