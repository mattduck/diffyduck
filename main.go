package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"duckdiff/aligner"
	"duckdiff/git"
	"duckdiff/parser"
	"duckdiff/ui"
)

func main() {
	// Check if we have a subcommand
	if len(os.Args) > 1 && os.Args[1] == "diff" {
		handleDiffCommand()
		return
	}

	// Default behavior: read from stdin
	input, err := readStdin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
		os.Exit(1)
	}

	// If stdin is empty, fall back to git diff
	if input == "" {
		input, err = getGitDiff()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting git diff: %v\n", err)
			os.Exit(1)
		}
		if input == "" {
			fmt.Println("no changes")
			os.Exit(0)
		}
	}

	runDiffViewer(input)
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

func handleDiffCommand() {
	input, err := getGitDiff()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting git diff: %v\n", err)
		os.Exit(1)
	}

	if input == "" {
		fmt.Println("no changes")
		os.Exit(0)
	}

	runDiffViewer(input)
}

func getGitDiff() (string, error) {
	cmd := exec.Command("git", "diff", "-U999999")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run git diff: %v", err)
	}

	return string(output), nil
}

func runDiffViewer(input string) {
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
		oldFileInfo, err := retriever.GetFileInfo(fileDiff.OldPath, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting old file info: %v\n", err)
			os.Exit(1)
		}

		newFileInfo, err := retriever.GetFileInfo(fileDiff.NewPath, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting new file info: %v\n", err)
			os.Exit(1)
		}

		var alignedLines []aligner.AlignedLine

		// Create alignment for non-binary files
		// Note: NewFile/DeletedFile types still contain text content that can be aligned
		isBinaryFile := oldFileInfo.Type == git.BinaryFile || newFileInfo.Type == git.BinaryFile
		if !isBinaryFile {
			alignedLines = diffAligner.AlignFile(oldFileInfo.Lines, newFileInfo.Lines, fileDiff.Hunks)
		}

		filesWithLines = append(filesWithLines, ui.FileWithLines{
			FileDiff:     fileDiff,
			AlignedLines: alignedLines,
			OldFileType:  oldFileInfo.Type,
			NewFileType:  newFileInfo.Type,
		})
	}

	model := ui.NewModel(filesWithLines)
	defer model.Close()

	p := tea.NewProgram(model)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}
