package pager

import (
	"io"
	"os"

	"github.com/mattn/go-isatty"
)

// IsStdinPipe returns true if stdin is a pipe (not a terminal).
// This is used to auto-detect pager mode.
func IsStdinPipe() bool {
	return !isatty.IsTerminal(os.Stdin.Fd()) && !isatty.IsCygwinTerminal(os.Stdin.Fd())
}

// ReadStdin reads all input from stdin and returns it as a string.
// It also strips any ANSI escape sequences from the input.
func ReadStdin() (string, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	return StripANSI(string(data)), nil
}
