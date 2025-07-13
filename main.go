package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"

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

	var allAlignedLines []aligner.AlignedLine
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
		allAlignedLines = append(allAlignedLines, alignedLines...)
	}

	if isatty.IsTerminal(os.Stdout.Fd()) {
		model := ui.NewModel(fileDiffs, allAlignedLines)
		p := tea.NewProgram(model)

		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
			os.Exit(1)
		}
	} else {
		for _, line := range allAlignedLines {
			var leftContent, rightContent string
			
			if line.OldLine != nil {
				leftContent = *line.OldLine
				if line.LineType == aligner.Deleted {
					leftContent = "-" + leftContent
				} else {
					leftContent = " " + leftContent
				}
			}
			
			if line.NewLine != nil {
				rightContent = *line.NewLine
				if line.LineType == aligner.Added {
					rightContent = "+" + rightContent
				} else {
					rightContent = " " + rightContent
				}
			}
			
			fmt.Printf("%-60s | %s\n", leftContent, rightContent)
		}
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