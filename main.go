package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"duckdiff/aligner"
	"duckdiff/git"
	"duckdiff/parser"
	"duckdiff/ui"
)

func main() {
	input, err := readStdin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
		os.Exit(1)
	}

	diffParser := parser.NewDiffParser()
	fileDiffs, err := diffParser.Parse(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing diff: %v\n", err)
		os.Exit(1)
	}

	var filesWithLines []ui.FileWithLines
	retriever := git.NewFileRetriever()
	diffAligner := aligner.NewDiffAligner()

	for _, fileDiff := range fileDiffs {
		oldLines, err := retriever.GetOldFileContent(fileDiff.OldPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting old file content: %v\n", err)
			os.Exit(1)
		}

		newLines, err := retriever.GetNewFileContent(fileDiff.NewPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting new file content: %v\n", err)
			os.Exit(1)
		}

		alignedLines := diffAligner.AlignFile(oldLines, newLines, fileDiff.Hunks)
		filesWithLines = append(filesWithLines, ui.FileWithLines{
			FileDiff: fileDiff,
			AlignedLines: alignedLines,
		})
	}

	model := ui.NewModel(filesWithLines)
	p := tea.NewProgram(model)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}

func readStdin() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	var result string
	
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			result += line
			break
		}
		if err != nil {
			return "", err
		}
		result += line
	}
	
	return result, nil
}