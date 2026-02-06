package git

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DefaultContext is the number of context lines to include around changes.
// With 8 lines on each side, hunks within ~16 lines of each other get merged.
const DefaultContext = 8

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

// command creates an exec.Cmd for a git invocation, configured for this
// RealGit's Dir. When Dir is set, the environment is sanitised to remove
// GIT_DIR, GIT_WORK_TREE, and GIT_INDEX_FILE so that inherited contexts
// (e.g. pre-commit hooks) cannot redirect commands to a different repo.
func (g *RealGit) command(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	if g.Dir != "" {
		cmd.Dir = g.Dir
		cmd.Env = cleanGitEnv(os.Environ())
	}
	return cmd
}

// commandWithEnv is like command but merges extra env vars on top.
// Used by CreateSnapshot which needs GIT_INDEX_FILE set.
func (g *RealGit) commandWithEnv(env []string, args ...string) *exec.Cmd {
	cmd := g.command(args...)
	cmd.Env = append(cmd.Env, env...)
	return cmd
}

// cleanGitEnv strips GIT_DIR, GIT_WORK_TREE, and GIT_INDEX_FILE from
// an environment slice so that commands target the intended repo.
func cleanGitEnv(environ []string) []string {
	clean := make([]string, 0, len(environ))
	for _, e := range environ {
		if strings.HasPrefix(e, "GIT_DIR=") ||
			strings.HasPrefix(e, "GIT_WORK_TREE=") ||
			strings.HasPrefix(e, "GIT_INDEX_FILE=") {
			continue
		}
		clean = append(clean, e)
	}
	return clean
}

// CommitMeta contains metadata about a git commit.
type CommitMeta struct {
	SHA     string // full commit hash
	Author  string // author name
	Email   string // author email
	Date    string // author date in ISO 8601 format
	Subject string // first line of commit message
	Body    string // rest of commit message
	Refs    string // raw ref decorations from %D (e.g., "HEAD -> main, origin/main")
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
	metaRefs        = "DIFFYDUCK_REFS:"
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
	metaRefs + "%D",
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
	metaRefs + "%D",
}, "%n") + "%n"

// Show returns the diff output for a given commit reference.
// Args are passed through to git show (e.g., ref, paths).
func (g *RealGit) Show(args ...string) (string, error) {
	gitArgs := append([]string{"show", "--format="}, prependContextFlag(args)...)
	cmd := g.command(gitArgs...)

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
	gitArgs := append([]string{"show", "--format=" + showMetaFormat}, prependContextFlag(args)...)
	cmd := g.command(gitArgs...)

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
func (g *RealGit) LogWithMeta(n int, args ...string) ([]CommitWithDiff, error) {
	gitArgs := []string{
		"log",
		"-p", // include patches
		fmt.Sprintf("-n%d", n),
		"--format=" + logMetaFormat,
	}
	gitArgs = append(gitArgs, args...)
	cmd := g.command(gitArgs...)

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

// LogMetaOnly returns commit metadata with per-file stats (no patches).
// Much faster than LogWithMeta for large histories since it doesn't fetch diff content.
func (g *RealGit) LogMetaOnly(n int, args ...string) ([]CommitWithStats, error) {
	return g.LogMetaOnlyRange(0, n, args...)
}

// LogMetaOnlyRange returns commit metadata with per-file stats for a range of commits.
// skip is the number of commits to skip from the start, limit is the max number to return.
func (g *RealGit) LogMetaOnlyRange(skip, limit int, args ...string) ([]CommitWithStats, error) {
	gitArgs := []string{
		"log",
		"--numstat", // gives "added<tab>removed<tab>path" per file
		fmt.Sprintf("-n%d", limit),
		"--format=" + logMetaFormat,
	}
	if skip > 0 {
		gitArgs = append(gitArgs, fmt.Sprintf("--skip=%d", skip))
	}
	gitArgs = append(gitArgs, args...)
	cmd := g.command(gitArgs...)

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

	return parseLogMetaOnly(string(out)), nil
}

// LogPathsOnly returns commit metadata with file paths only (no stats or patches).
// This is the fastest option for large histories since git only needs to list files.
// Use LogMetaOnly or fetch stats separately when per-file stats are needed.
func (g *RealGit) LogPathsOnly(n int, args ...string) ([]CommitWithPaths, error) {
	gitArgs := []string{
		"log",
		"--name-only", // gives just file paths, no stats
		fmt.Sprintf("-n%d", n),
		"--format=" + logMetaFormat,
	}
	gitArgs = append(gitArgs, args...)
	cmd := g.command(gitArgs...)

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

	return parseLogPathsOnly(string(out)), nil
}

// LogPathsOnlyRange returns commit metadata with file paths for a range of commits.
// skip is the number of commits to skip from the start, limit is the max number to return.
func (g *RealGit) LogPathsOnlyRange(skip, limit int, args ...string) ([]CommitWithPaths, error) {
	gitArgs := []string{
		"log",
		"--name-only",
		fmt.Sprintf("-n%d", limit),
		"--format=" + logMetaFormat,
	}
	if skip > 0 {
		gitArgs = append(gitArgs, fmt.Sprintf("--skip=%d", skip))
	}
	gitArgs = append(gitArgs, args...)
	cmd := g.command(gitArgs...)

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

	return parseLogPathsOnly(string(out)), nil
}

// CommitCount returns the total number of commits in the repository.
// Uses git rev-list --count HEAD which is fast even on large repos.
func (g *RealGit) CommitCount() (int, error) {
	cmd := g.command("rev-list", "--count", "HEAD")

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return -1, &GitError{
				Command: "git rev-list --count",
				Stderr:  strings.TrimSpace(string(exitErr.Stderr)),
			}
		}
		return -1, err
	}

	count := 0
	_, err = fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &count)
	if err != nil {
		return -1, err
	}
	return count, nil
}

// parseLogPathsOnly parses git log --name-only output into commits with paths.
func parseLogPathsOnly(output string) []CommitWithPaths {
	var results []CommitWithPaths

	// Split by commit start marker
	parts := strings.Split(output, metaCommitStart+"\n")

	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}

		meta, files := parsePathsOnlyOutput(part)
		if meta.SHA != "" {
			results = append(results, CommitWithPaths{
				Meta:  meta,
				Files: files,
			})
		}
	}

	return results
}

// parsePathsOnlyOutput parses a single commit's metadata and name-only output.
func parsePathsOnlyOutput(output string) (*CommitMeta, []FilePath) {
	meta := &CommitMeta{}
	var files []FilePath
	var bodyLines []string
	inBody := false

	lines := strings.Split(output, "\n")

	for _, line := range lines {
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
		case strings.HasPrefix(line, metaRefs):
			meta.Refs = strings.TrimPrefix(line, metaRefs)
		case inBody:
			bodyLines = append(bodyLines, line)
		default:
			// Non-empty lines after body are file paths
			if line != "" {
				files = append(files, FilePath{Path: line})
			}
		}
	}

	meta.Body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
	return meta, files
}

// parseLogMetaOnly parses git log --numstat output into commits with stats.
func parseLogMetaOnly(output string) []CommitWithStats {
	var results []CommitWithStats

	// Split by commit start marker
	parts := strings.Split(output, metaCommitStart+"\n")

	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}

		meta, files := parseMetaOnlyOutput(part)
		if meta.SHA != "" {
			results = append(results, CommitWithStats{
				Meta:  meta,
				Files: files,
			})
		}
	}

	return results
}

// parseMetaOnlyOutput parses a single commit's metadata and numstat output.
func parseMetaOnlyOutput(output string) (*CommitMeta, []FileStats) {
	meta := &CommitMeta{}
	var files []FileStats
	var bodyLines []string
	inBody := false

	lines := strings.Split(output, "\n")

	for _, line := range lines {
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
		case strings.HasPrefix(line, metaRefs):
			meta.Refs = strings.TrimPrefix(line, metaRefs)
		case inBody:
			bodyLines = append(bodyLines, line)
		default:
			// Try to parse as numstat line: "added<tab>removed<tab>path"
			if fs := parseNumstatLine(line); fs != nil {
				files = append(files, *fs)
			}
		}
	}

	meta.Body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
	return meta, files
}

// parseNumstatLine parses a single numstat line like "10\t5\tpath/to/file".
// Returns nil if the line is not a valid numstat line.
func parseNumstatLine(line string) *FileStats {
	if line == "" {
		return nil
	}

	parts := strings.Split(line, "\t")
	if len(parts) != 3 {
		return nil
	}

	added := -1 // -1 indicates binary
	removed := -1

	if parts[0] != "-" {
		fmt.Sscanf(parts[0], "%d", &added)
	}
	if parts[1] != "-" {
		fmt.Sscanf(parts[1], "%d", &removed)
	}

	return &FileStats{
		Path:    parts[2],
		Added:   added,
		Removed: removed,
	}
}

// CommitWithDiff holds a commit's metadata and its diff output.
type CommitWithDiff struct {
	Meta *CommitMeta
	Diff string
}

// FileStats holds per-file statistics from git log --numstat.
type FileStats struct {
	Path    string
	Added   int // -1 for binary files
	Removed int // -1 for binary files
}

// CommitWithStats holds a commit's metadata and per-file stats (no patch content).
type CommitWithStats struct {
	Meta  *CommitMeta
	Files []FileStats
}

// FilePath holds just a file path from git log --name-only.
type FilePath struct {
	Path string
}

// CommitWithPaths holds a commit's metadata and file paths (no stats or patch content).
// This is faster to fetch than CommitWithStats since git doesn't need to compute diffs.
type CommitWithPaths struct {
	Meta  *CommitMeta
	Files []FilePath
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
		case strings.HasPrefix(line, metaRefs):
			meta.Refs = strings.TrimPrefix(line, metaRefs)
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
	gitArgs := append([]string{"diff"}, prependContextFlag(args)...)
	cmd := g.command(gitArgs...)

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

// HasConflicts returns true if the repo is in a merge, rebase, or cherry-pick
// state by checking for sentinel files in the git directory.
func (g *RealGit) HasConflicts() bool {
	out, err := g.command("rev-parse", "--git-dir").Output()
	if err != nil {
		return false
	}
	gitDir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(gitDir) && g.Dir != "" {
		gitDir = filepath.Join(g.Dir, gitDir)
	}
	sentinels := []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "rebase-merge", "rebase-apply"}
	for _, s := range sentinels {
		if _, err := os.Stat(filepath.Join(gitDir, s)); err == nil {
			return true
		}
	}
	return false
}

// GetFileContent returns the content of a file at a given ref.
// Uses git show <ref>:<path> to retrieve the content.
// If ref is empty, retrieves from the index (staged content).
func (g *RealGit) GetFileContent(ref, path string) (string, error) {
	// Build the ref:path specifier
	specifier := ref + ":" + path

	cmd := g.command("show", specifier)

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

	cmd := g.command("show", specifier)

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
	cmd := g.command("ls-files", "--others", "--exclude-standard")

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
	cmd := g.command("diff", "--no-index", "/dev/null", path)

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

// prependContextFlag adds -U<DefaultContext> to args unless the user already specified a -U flag.
func prependContextFlag(args []string) []string {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-U") || strings.HasPrefix(arg, "--unified") {
			return args
		}
	}
	return append([]string{fmt.Sprintf("-U%d", DefaultContext)}, args...)
}

// CreateSnapshot creates a commit representing the current working tree state.
// Uses a temporary index file to avoid affecting the real index.
// If allMode is true, includes untracked files (-A); otherwise only tracked files (-u).
// If parentSHA is non-empty, the commit will have that as its parent, forming a chain.
// The message is used as the commit message.
func (g *RealGit) CreateSnapshot(allMode bool, parentSHA string, message string) (string, error) {
	// Create a temporary index file
	tmpDir := os.TempDir()
	tmpIndex := filepath.Join(tmpDir, fmt.Sprintf("dfd-snapshot-%d", os.Getpid()))
	defer os.Remove(tmpIndex)

	// Copy the current index to the temp file so we start from the current staged state
	// First, get the git dir to find the real index
	cmd := g.command("rev-parse", "--git-dir")
	gitDirOut, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get git dir: %w", err)
	}
	gitDir := strings.TrimSpace(string(gitDirOut))
	if !filepath.IsAbs(gitDir) && g.Dir != "" {
		gitDir = filepath.Join(g.Dir, gitDir)
	}

	// Read the real index and copy it (if it exists)
	realIndex := filepath.Join(gitDir, "index")
	if indexData, err := os.ReadFile(realIndex); err == nil {
		if err := os.WriteFile(tmpIndex, indexData, 0600); err != nil {
			return "", fmt.Errorf("copy index: %w", err)
		}
	}

	// Add files to the temporary index
	addFlag := "-u" // only tracked files
	if allMode {
		addFlag = "-A" // include untracked
	}
	cmd = g.commandWithEnv([]string{"GIT_INDEX_FILE=" + tmpIndex}, "add", addFlag)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add %s: %s", addFlag, strings.TrimSpace(string(out)))
	}

	// Write the tree
	cmd = g.commandWithEnv([]string{"GIT_INDEX_FILE=" + tmpIndex}, "write-tree")
	treeOut, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git write-tree: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("git write-tree: %w", err)
	}
	treeSHA := strings.TrimSpace(string(treeOut))

	// Create the commit, optionally with a parent
	args := []string{"commit-tree", treeSHA, "-m", message}
	if parentSHA != "" {
		args = []string{"commit-tree", treeSHA, "-p", parentSHA, "-m", message}
	}
	cmd = g.command(args...)
	commitOut, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git commit-tree: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("git commit-tree: %w", err)
	}

	return strings.TrimSpace(string(commitOut)), nil
}

// DiffSnapshots returns the diff between two snapshot commits.
func (g *RealGit) DiffSnapshots(sha1, sha2 string, args ...string) (string, error) {
	allArgs := []string{sha1, sha2}
	allArgs = append(allArgs, args...)
	return g.Diff(allArgs...)
}

// snapshotRefPrefix is the prefix for persisted snapshot refs.
const snapshotRefPrefix = "refs/dfd/snapshots/"

// UpdateSnapshotRef updates refs/dfd/snapshots/<baseSHA> to point to sha.
func (g *RealGit) UpdateSnapshotRef(baseSHA string, sha string) error {
	refName := fmt.Sprintf("%s%s", snapshotRefPrefix, baseSHA)
	cmd := g.command("update-ref", refName, sha)

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git update-ref %s: %s", refName, strings.TrimSpace(string(out)))
	}
	return nil
}

// ListSnapshotRefs returns snapshot info for all snapshots for a base SHA.
// Uses git log to traverse the parent chain from refs/dfd/snapshots/<baseSHA>.
// Returns oldest first (chronological order).
func (g *RealGit) ListSnapshotRefs(baseSHA string) ([]SnapshotInfo, error) {
	refName := fmt.Sprintf("%s%s", snapshotRefPrefix, baseSHA)

	// Check if the ref exists first
	checkCmd := g.command("rev-parse", "--verify", refName)
	if err := checkCmd.Run(); err != nil {
		// Ref doesn't exist - no snapshots
		return nil, nil
	}

	// Use git log to traverse parent chain, stopping at baseSHA
	// --ancestry-path ensures we only follow the direct line
	// Format: SHA<NUL>subject<NUL>date, one record per line
	cmd := g.command("log", "--format=%H%x00%s%x00%ci", "--ancestry-path", baseSHA+".."+refName)

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

	output := strings.TrimSpace(string(out))
	if output == "" {
		return nil, nil
	}

	// Split into lines - git log returns newest first
	lines := strings.Split(output, "\n")

	// Parse each line and reverse to get oldest first
	result := make([]SnapshotInfo, len(lines))
	for i, line := range lines {
		parts := strings.SplitN(line, "\x00", 3)
		info := SnapshotInfo{}
		if len(parts) >= 1 {
			info.SHA = parts[0]
		}
		if len(parts) >= 2 {
			info.Subject = parts[1]
		}
		if len(parts) >= 3 {
			// Parse and reformat date to compact form
			dateStr := parts[2]
			if t, err := time.Parse("2006-01-02 15:04:05 -0700", dateStr); err == nil {
				info.Date = t.Format("Jan 2 15:04")
			} else {
				info.Date = dateStr
			}
		}
		// Reverse order: oldest first
		result[len(lines)-1-i] = info
	}

	return result, nil
}

// DeleteSnapshotRefs deletes snapshot refs under refs/dfd/snapshots/.
// If baseSHA is non-empty, only deletes that specific ref; otherwise deletes all.
func (g *RealGit) DeleteSnapshotRefs(baseSHA string) error {
	if baseSHA != "" {
		// Delete single ref for this base
		refName := fmt.Sprintf("%s%s", snapshotRefPrefix, baseSHA)
		cmd := g.command("update-ref", "-d", refName)
		// Ignore error if ref doesn't exist
		_ = cmd.Run()
		return nil
	}

	// Delete all refs under the prefix
	cmd := g.command("for-each-ref", "--format=%(refname)", snapshotRefPrefix)

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &GitError{
				Command: "git for-each-ref",
				Stderr:  strings.TrimSpace(string(exitErr.Stderr)),
			}
		}
		return err
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		return nil // nothing to delete
	}

	// Delete each ref
	refs := strings.Split(output, "\n")
	for _, ref := range refs {
		if ref == "" {
			continue
		}
		delCmd := g.command("update-ref", "-d", ref)
		if out, err := delCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git update-ref -d %s: %s", ref, strings.TrimSpace(string(out)))
		}
	}

	return nil
}

// ExpireOldSnapshotRefs deletes snapshot refs older than maxAgeDays.
// Returns the number of deleted refs.
func (g *RealGit) ExpireOldSnapshotRefs(maxAgeDays int) (int, error) {
	// Get refs with commit dates using for-each-ref
	cmd := g.command("for-each-ref",
		"--format=%(refname):%(committerdate:unix)",
		snapshotRefPrefix)

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return 0, &GitError{
				Command: "git for-each-ref",
				Stderr:  strings.TrimSpace(string(exitErr.Stderr)),
			}
		}
		return 0, err
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		return 0, nil // no refs to expire
	}

	// Calculate cutoff time
	cutoff := time.Now().AddDate(0, 0, -maxAgeDays).Unix()

	// Parse output and collect refs to delete
	var refsToDelete []string
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		refName := parts[0]
		var timestamp int64
		if _, err := fmt.Sscanf(parts[1], "%d", &timestamp); err != nil {
			continue
		}

		if timestamp < cutoff {
			refsToDelete = append(refsToDelete, refName)
		}
	}

	// Delete old refs
	deleted := 0
	for _, ref := range refsToDelete {
		cmd := g.command("update-ref", "-d", ref)
		if _, err := cmd.CombinedOutput(); err == nil {
			deleted++
		}
	}

	return deleted, nil
}

// LocalBranches returns all local branches with tip commit metadata.
func (g *RealGit) LocalBranches() ([]BranchInfo, error) {
	// NUL-delimited fields for reliable parsing
	format := "%(refname:short)%00%(objectname)%00%(subject)%00%(authordate:iso-strict)%00%(authorname)%00%(HEAD)"
	cmd := g.command("for-each-ref", "--format="+format, "--sort=-authordate", "refs/heads/")

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, &GitError{
				Command: "git for-each-ref",
				Stderr:  strings.TrimSpace(string(exitErr.Stderr)),
			}
		}
		return nil, err
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		return nil, nil
	}

	var branches []BranchInfo
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(line, "\x00", 6)
		if len(parts) < 6 {
			continue
		}
		branches = append(branches, BranchInfo{
			Name:    parts[0],
			SHA:     parts[1],
			Subject: parts[2],
			Date:    parts[3],
			Author:  parts[4],
			IsHead:  strings.TrimSpace(parts[5]) == "*",
		})
	}

	return branches, nil
}

// MergeBase returns the best common ancestor SHA of two refs.
// Returns empty string and no error if there is no common ancestor.
func (g *RealGit) MergeBase(a, b string) (string, error) {
	cmd := g.command("merge-base", a, b)

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 1 means no common ancestor
			if exitErr.ExitCode() == 1 {
				return "", nil
			}
			return "", &GitError{
				Command: "git merge-base",
				Stderr:  strings.TrimSpace(string(exitErr.Stderr)),
			}
		}
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

// AheadBehind returns how many commits a is ahead of and behind b.
func (g *RealGit) AheadBehind(a, b string) (ahead, behind int, err error) {
	cmd := g.command("rev-list", "--left-right", "--count", a+"..."+b)

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return 0, 0, &GitError{
				Command: "git rev-list --left-right --count",
				Stderr:  strings.TrimSpace(string(exitErr.Stderr)),
			}
		}
		return 0, 0, err
	}

	_, scanErr := fmt.Sscanf(strings.TrimSpace(string(out)), "%d\t%d", &ahead, &behind)
	if scanErr != nil {
		return 0, 0, fmt.Errorf("parse rev-list output: %w", scanErr)
	}

	return ahead, behind, nil
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
