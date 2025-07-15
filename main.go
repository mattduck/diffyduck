package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mattduck/diffyduck/aligner"
	"github.com/mattduck/diffyduck/git"
	"github.com/mattduck/diffyduck/parser"
	"github.com/mattduck/diffyduck/ui"
)

func main() {
	// Check if we have a subcommand
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "diff":
			handleDiffCommand()
			return
		case "show":
			handleShowCommand()
			return
		case "pager":
			handlePagerCommand()
			return
		}
	}

	// Default behavior: run git diff
	handleDiffCommand()
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

func handlePagerCommand() {
	// Read from stdin
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

func handleDiffCommand() {
	// Pass all arguments after "diff" to git diff
	var diffArgs []string

	// Check if we were called as a subcommand or as the default
	if len(os.Args) > 1 && os.Args[1] == "diff" && len(os.Args) > 2 {
		diffArgs = os.Args[2:]
	} else if len(os.Args) > 1 && os.Args[1] != "diff" {
		// Default behavior with args passed directly
		diffArgs = os.Args[1:]
	}

	input, err := getGitDiff(diffArgs...)
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

func getGitDiff(args ...string) (string, error) {
	cmdArgs := []string{"diff", "-U999999"}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("git", cmdArgs...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run git diff: %v", err)
	}

	return string(output), nil
}

// handleShowCommand processes the 'show' subcommand, which displays the diff of a specific commit
// against its parent. It accepts the same arguments as 'git show' and passes them through.
// Examples:
//
//	diffyduck show         -> shows HEAD commit
//	diffyduck show HEAD~1  -> shows HEAD~1 commit
//	diffyduck show --help  -> shows git show help
func handleShowCommand() {
	// Pass all arguments after "show" to git show
	var commitArgs []string
	if len(os.Args) > 2 {
		commitArgs = os.Args[2:]
	} else {
		// Default to HEAD if no commit specified
		commitArgs = []string{"HEAD"}
	}

	gitArgs := []string{"-U999999"}
	gitArgs = append(gitArgs, commitArgs...)

	input, err := getGitShow(gitArgs...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting git show: %v\n", err)
		os.Exit(1)
	}

	if input == "" {
		fmt.Println("no changes")
		os.Exit(0)
	}

	runShowViewer(input, commitArgs...)
}

// getGitShow executes 'git show' with the provided arguments and returns the diff output.
// It automatically adds the -U999999 flag for maximum context lines to match the behavior
// of the diff command. All git show arguments and flags are supported.
func getGitShow(args ...string) (string, error) {
	cmdArgs := []string{"show"}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("git", cmdArgs...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run git show: %v", err)
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

// runShowViewer parses git show output and displays it in the interactive diff viewer.
// It retrieves file content from the specified commit and its parent using git show,
// then renders the diff with syntax highlighting and alignment.
// commitArgs contains the commit specification (defaults to HEAD if empty).
func runShowViewer(input string, commitArgs ...string) {
	diffParser := parser.NewDiffParser()
	fileDiffs, err := diffParser.Parse(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing diff: %v\n", err)
		os.Exit(1)
	}

	var filesWithLines []ui.FileWithLines
	retriever := git.NewFileRetriever()
	diffAligner := aligner.NewDiffAligner()

	// Determine the commit being shown (default to HEAD)
	commit := "HEAD"
	if len(commitArgs) > 0 {
		commit = commitArgs[0]
	}

	for _, fileDiff := range fileDiffs {
		oldFileInfo, err := retriever.GetFileInfoFromCommit(fileDiff.OldPath, commit+"~1")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting old file info: %v\n", err)
			os.Exit(1)
		}

		newFileInfo, err := retriever.GetFileInfoFromCommit(fileDiff.NewPath, commit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting new file info: %v\n", err)
			os.Exit(1)
		}

		var alignedLines []aligner.AlignedLine

		// Create alignment for non-binary files
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
