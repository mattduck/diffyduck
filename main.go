package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"duckdiff/display"
	"duckdiff/parser"
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

	renderer := display.NewRenderer()
	err = renderer.RenderFileDiffs(fileDiffs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering diff: %v\n", err)
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