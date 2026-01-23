package git

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// RealGit implements Git by executing actual git commands.
type RealGit struct {
	// Dir is the working directory for git commands.
	// If empty, uses the current directory.
	Dir string
}

// New creates a new RealGit instance.
func New() *RealGit {
	return &RealGit{}
}

// NewWithDir creates a new RealGit instance with a specific directory.
func NewWithDir(dir string) *RealGit {
	return &RealGit{Dir: dir}
}

// CommitMeta contains metadata about a git commit.
type CommitMeta struct {
	SHA     string // full commit hash
	Author  string // author name
	Email   string // author email
	Date    string // author date in ISO 8601 format
	Subject string // first line of commit message
	Body    string // rest of commit message
}

// Delimiters used in custom format output for reliable parsing.
const (
	metaCommitStart = "DIFFYDUCK_COMMIT_START"
	metaSHA         = "DIFFYDUCK_SHA:"
	metaAuthor      = "DIFFYDUCK_AUTHOR:"
	metaEmail       = "DIFFYDUCK_EMAIL:"
	metaDate        = "DIFFYDUCK_DATE:"
	metaSubject     = "DIFFYDUCK_SUBJECT:"
	metaBodyStart   = "DIFFYDUCK_BODY_START"
	metaBodyEnd     = "DIFFYDUCK_BODY_END"
)

// showMetaFormat is the git format string for extracting commit metadata.
var showMetaFormat = strings.Join([]string{
	metaSHA + "%H",
	metaAuthor + "%an",
	metaEmail + "%ae",
	metaDate + "%aI",
	metaSubject + "%s",
	metaBodyStart,
	"%b",
	metaBodyEnd,
}, "%n") + "%n"

// logMetaFormat is the git format string for log with commit boundary markers.
var logMetaFormat = strings.Join([]string{
	metaCommitStart,
	metaSHA + "%H",
	metaAuthor + "%an",
	metaEmail + "%ae",
	metaDate + "%aI",
	metaSubject + "%s",
	metaBodyStart,
	"%b",
	metaBodyEnd,
}, "%n") + "%n"

// Show returns the diff output for a given commit reference.
// Args are passed through to git show (e.g., ref, paths).
func (g *RealGit) Show(args ...string) (string, error) {
	gitArgs := append([]string{"show", "--format="}, args...)
	cmd := exec.Command("git", gitArgs...)
	if g.Dir != "" {
		cmd.Dir = g.Dir
	}

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", &GitError{
				Command: "git show",
				Stderr:  strings.TrimSpace(string(exitErr.Stderr)),
			}
		}
		return "", err
	}

	return string(out), nil
}

// ShowWithMeta returns both commit metadata and diff output for a given commit.
// The first return value is the parsed commit metadata.
// The second return value is the diff output (starting from "diff --git").
func (g *RealGit) ShowWithMeta(args ...string) (*CommitMeta, string, error) {
	gitArgs := append([]string{"show", "--format=" + showMetaFormat}, args...)
	cmd := exec.Command("git", gitArgs...)
	if g.Dir != "" {
		cmd.Dir = g.Dir
	}

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, "", &GitError{
				Command: "git show",
				Stderr:  strings.TrimSpace(string(exitErr.Stderr)),
			}
		}
		return nil, "", err
	}

	meta, diff := parseShowOutput(string(out))
	return meta, diff, nil
}

// LogWithMeta returns commit metadata and diff output for multiple commits.
// The n parameter limits the number of commits returned.
// Returns a slice of (CommitMeta, diff string) pairs.
func (g *RealGit) LogWithMeta(n int) ([]CommitWithDiff, error) {
	gitArgs := []string{
		"log",
		"-p", // include patches
		fmt.Sprintf("-n%d", n),
		"--format=" + logMetaFormat,
	}
	cmd := exec.Command("git", gitArgs...)
	if g.Dir != "" {
		cmd.Dir = g.Dir
	}

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, &GitError{
				Command: "git log",
				Stderr:  strings.TrimSpace(string(exitErr.Stderr)),
			}
		}
		return nil, err
	}

	return parseLogOutput(string(out)), nil
}

// CommitWithDiff holds a commit's metadata and its diff output.
type CommitWithDiff struct {
	Meta *CommitMeta
	Diff string
}

// parseLogOutput splits git log output into multiple commits.
func parseLogOutput(output string) []CommitWithDiff {
	var results []CommitWithDiff

	// Split by commit start marker
	parts := strings.Split(output, metaCommitStart+"\n")

	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}

		meta, diff := parseShowOutput(part)
		if meta.SHA != "" {
			results = append(results, CommitWithDiff{
				Meta: meta,
				Diff: diff,
			})
		}
	}

	return results
}

// parseShowOutput splits git show output into metadata and diff portions.
func parseShowOutput(output string) (*CommitMeta, string) {
	meta := &CommitMeta{}
	lines := strings.Split(output, "\n")

	var bodyLines []string
	inBody := false
	diffStartIdx := -1

	for i, line := range lines {
		// Check for diff start
		if strings.HasPrefix(line, "diff --git") {
			diffStartIdx = i
			break
		}

		// Parse metadata fields
		switch {
		case strings.HasPrefix(line, metaSHA):
			meta.SHA = strings.TrimPrefix(line, metaSHA)
		case strings.HasPrefix(line, metaAuthor):
			meta.Author = strings.TrimPrefix(line, metaAuthor)
		case strings.HasPrefix(line, metaEmail):
			meta.Email = strings.TrimPrefix(line, metaEmail)
		case strings.HasPrefix(line, metaDate):
			meta.Date = strings.TrimPrefix(line, metaDate)
		case strings.HasPrefix(line, metaSubject):
			meta.Subject = strings.TrimPrefix(line, metaSubject)
		case line == metaBodyStart:
			inBody = true
		case line == metaBodyEnd:
			inBody = false
		case inBody:
			bodyLines = append(bodyLines, line)
		}
	}

	meta.Body = strings.TrimSpace(strings.Join(bodyLines, "\n"))

	// Extract diff portion
	var diff string
	if diffStartIdx >= 0 {
		diff = strings.Join(lines[diffStartIdx:], "\n")
	}

	return meta, diff
}

// Diff returns the diff output.
// Args are passed through to git diff (e.g., --cached, refs, paths).
func (g *RealGit) Diff(args ...string) (string, error) {
	gitArgs := append([]string{"diff"}, args...)
	cmd := exec.Command("git", gitArgs...)
	if g.Dir != "" {
		cmd.Dir = g.Dir
	}

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", &GitError{
				Command: "git diff",
				Stderr:  strings.TrimSpace(string(exitErr.Stderr)),
			}
		}
		return "", err
	}

	return string(out), nil
}

// GetFileContent returns the content of a file at a given ref.
// Uses git show <ref>:<path> to retrieve the content.
// If ref is empty, retrieves from the index (staged content).
func (g *RealGit) GetFileContent(ref, path string) (string, error) {
	// Build the ref:path specifier
	specifier := ref + ":" + path

	cmd := exec.Command("git", "show", specifier)
	if g.Dir != "" {
		cmd.Dir = g.Dir
	}

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", &GitError{
				Command: "git show " + specifier,
				Stderr:  strings.TrimSpace(string(exitErr.Stderr)),
			}
		}
		return "", err
	}

	return string(out), nil
}

// GetFileContentReader returns a reader for streaming file content at a given ref.
// The caller must close the returned ReadCloser when done.
// The cleanup function must be called after closing the reader to wait for the git process.
func (g *RealGit) GetFileContentReader(ref, path string) (io.ReadCloser, func() error, error) {
	specifier := ref + ":" + path

	cmd := exec.Command("git", "show", specifier)
	if g.Dir != "" {
		cmd.Dir = g.Dir
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	cleanup := func() error {
		return cmd.Wait()
	}

	return stdout, cleanup, nil
}

// ListUntrackedFiles returns a list of untracked files (excluding ignored files).
func (g *RealGit) ListUntrackedFiles() ([]string, error) {
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	if g.Dir != "" {
		cmd.Dir = g.Dir
	}

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, &GitError{
				Command: "git ls-files",
				Stderr:  strings.TrimSpace(string(exitErr.Stderr)),
			}
		}
		return nil, err
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		return nil, nil
	}

	return strings.Split(output, "\n"), nil
}

// DiffNewFile generates a diff showing a file as entirely new.
// Uses git diff --no-index to compare /dev/null against the file.
func (g *RealGit) DiffNewFile(path string) (string, error) {
	cmd := exec.Command("git", "diff", "--no-index", "/dev/null", path)
	if g.Dir != "" {
		cmd.Dir = g.Dir
	}

	out, err := cmd.Output()
	// git diff --no-index returns exit code 1 when files differ, which is expected
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 1 is normal for diffs with changes
			if exitErr.ExitCode() == 1 {
				return string(out), nil
			}
			return "", &GitError{
				Command: "git diff --no-index",
				Stderr:  strings.TrimSpace(string(exitErr.Stderr)),
			}
		}
		return "", err
	}

	return string(out), nil
}

// GitError represents an error from a git command.
type GitError struct {
	Command string
	Stderr  string
}

func (e *GitError) Error() string {
	if e.Stderr != "" {
		return e.Command + ": " + e.Stderr
	}
	return e.Command + " failed"
}
