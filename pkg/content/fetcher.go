package content

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/user/diffyduck/pkg/git"
)

// Mode represents the diff mode which determines how to fetch file content.
type Mode int

const (
	// ModeShow is for git show <commit> - compare commit to parent.
	ModeShow Mode = iota
	// ModeDiffUnstaged is for git diff - compare index to working tree.
	ModeDiffUnstaged
	// ModeDiffCached is for git diff --cached - compare HEAD to index.
	ModeDiffCached
	// ModeDiffRefs is for git diff <ref1> <ref2> - compare two refs.
	ModeDiffRefs
)

// Fetcher retrieves file content from git based on the diff mode.
type Fetcher struct {
	git  git.Git
	mode Mode
	ref1 string // For ModeShow: the commit. For ModeDiffRefs: first ref.
	ref2 string // For ModeDiffRefs: second ref.

	// WorkDir is the working directory for reading working tree files.
	// If empty, uses the current directory.
	WorkDir string

	// cache stores fetched content to avoid repeated git calls.
	// Key format: "old:<path>" or "new:<path>"
	cache   map[string]string
	cacheMu sync.RWMutex

	// linesCache stores limited line content.
	// Key format: "old:<path>" or "new:<path>"
	linesCache   map[string]*linesResult
	linesCacheMu sync.RWMutex

	// onFetch is called when content is fetched (for testing).
	onFetch func()
}

// linesResult holds the result of a limited lines fetch.
type linesResult struct {
	lines     []string
	truncated bool
}

// NewFetcher creates a new content fetcher.
// For ModeShow, ref1 is the commit ref.
// For ModeDiffRefs, ref1 and ref2 are the refs to compare.
// For ModeDiffUnstaged and ModeDiffCached, refs are ignored.
func NewFetcher(g git.Git, mode Mode, ref1, ref2 string) *Fetcher {
	return &Fetcher{
		git:        g,
		mode:       mode,
		ref1:       ref1,
		ref2:       ref2,
		cache:      make(map[string]string),
		linesCache: make(map[string]*linesResult),
	}
}

// GetOldContent returns the old version of a file.
func (f *Fetcher) GetOldContent(path string) (string, error) {
	cacheKey := "old:" + path

	// Check cache with read lock
	f.cacheMu.RLock()
	if content, ok := f.cache[cacheKey]; ok {
		f.cacheMu.RUnlock()
		return content, nil
	}
	f.cacheMu.RUnlock()

	content, err := f.fetchOld(path)
	if err != nil {
		// Check if this is a "file not found" error (new file case)
		if isFileNotFoundError(err) {
			f.cacheMu.Lock()
			f.cache[cacheKey] = ""
			f.cacheMu.Unlock()
			return "", nil
		}
		return "", err
	}

	if f.onFetch != nil {
		f.onFetch()
	}

	f.cacheMu.Lock()
	f.cache[cacheKey] = content
	f.cacheMu.Unlock()
	return content, nil
}

// GetNewContent returns the new version of a file.
func (f *Fetcher) GetNewContent(path string) (string, error) {
	cacheKey := "new:" + path

	// Check cache with read lock
	f.cacheMu.RLock()
	if content, ok := f.cache[cacheKey]; ok {
		f.cacheMu.RUnlock()
		return content, nil
	}
	f.cacheMu.RUnlock()

	content, err := f.fetchNew(path)
	if err != nil {
		// Check if this is a "file not found" error (deleted file case)
		if isFileNotFoundError(err) {
			f.cacheMu.Lock()
			f.cache[cacheKey] = ""
			f.cacheMu.Unlock()
			return "", nil
		}
		return "", err
	}

	if f.onFetch != nil {
		f.onFetch()
	}

	f.cacheMu.Lock()
	f.cache[cacheKey] = content
	f.cacheMu.Unlock()
	return content, nil
}

// GetOldContentLines returns the old version of a file as lines with limits applied.
// Returns lines, whether content was truncated, and any error.
// Truncation occurs if byte limit, line limit, or line length limit is exceeded.
func (f *Fetcher) GetOldContentLines(path string) ([]string, bool, error) {
	cacheKey := "old:" + path

	// Check cache with read lock
	f.linesCacheMu.RLock()
	if result, ok := f.linesCache[cacheKey]; ok {
		f.linesCacheMu.RUnlock()
		return result.lines, result.truncated, nil
	}
	f.linesCacheMu.RUnlock()

	lines, truncated, err := f.fetchOldLines(path)
	if err != nil {
		if isFileNotFoundError(err) {
			f.linesCacheMu.Lock()
			f.linesCache[cacheKey] = &linesResult{lines: nil, truncated: false}
			f.linesCacheMu.Unlock()
			return nil, false, nil
		}
		return nil, false, err
	}

	if f.onFetch != nil {
		f.onFetch()
	}

	f.linesCacheMu.Lock()
	f.linesCache[cacheKey] = &linesResult{lines: lines, truncated: truncated}
	f.linesCacheMu.Unlock()
	return lines, truncated, nil
}

// GetNewContentLines returns the new version of a file as lines with limits applied.
// Returns lines, whether content was truncated, and any error.
// Truncation occurs if byte limit, line limit, or line length limit is exceeded.
func (f *Fetcher) GetNewContentLines(path string) ([]string, bool, error) {
	cacheKey := "new:" + path

	// Check cache with read lock
	f.linesCacheMu.RLock()
	if result, ok := f.linesCache[cacheKey]; ok {
		f.linesCacheMu.RUnlock()
		return result.lines, result.truncated, nil
	}
	f.linesCacheMu.RUnlock()

	lines, truncated, err := f.fetchNewLines(path)
	if err != nil {
		if isFileNotFoundError(err) {
			f.linesCacheMu.Lock()
			f.linesCache[cacheKey] = &linesResult{lines: nil, truncated: false}
			f.linesCacheMu.Unlock()
			return nil, false, nil
		}
		return nil, false, err
	}

	if f.onFetch != nil {
		f.onFetch()
	}

	f.linesCacheMu.Lock()
	f.linesCache[cacheKey] = &linesResult{lines: lines, truncated: truncated}
	f.linesCacheMu.Unlock()
	return lines, truncated, nil
}

func (f *Fetcher) fetchOldLines(path string) ([]string, bool, error) {
	var ref string
	switch f.mode {
	case ModeShow:
		ref = f.ref1 + "^"
	case ModeDiffUnstaged:
		ref = ""
	case ModeDiffCached:
		ref = "HEAD"
	case ModeDiffRefs:
		ref = f.ref1
	default:
		ref = "HEAD"
	}
	return f.fetchGitContentLines(ref, path)
}

func (f *Fetcher) fetchNewLines(path string) ([]string, bool, error) {
	switch f.mode {
	case ModeShow:
		return f.fetchGitContentLines(f.ref1, path)
	case ModeDiffUnstaged:
		// Read from working tree
		return f.readWorkingTreeFileLines(path)
	case ModeDiffCached:
		return f.fetchGitContentLines("", path)
	case ModeDiffRefs:
		return f.fetchGitContentLines(f.ref2, path)
	default:
		return f.fetchGitContentLines("HEAD", path)
	}
}

func (f *Fetcher) fetchGitContentLines(ref, path string) ([]string, bool, error) {
	reader, cleanup, err := f.git.GetFileContentReader(ref, path)
	if err != nil {
		return nil, false, err
	}
	defer reader.Close()
	defer cleanup()

	return ReadLimitedLines(reader)
}

func (f *Fetcher) readWorkingTreeFileLines(path string) ([]string, bool, error) {
	fullPath := path
	if f.WorkDir != "" {
		fullPath = filepath.Join(f.WorkDir, path)
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()

	return ReadLimitedLines(file)
}

func (f *Fetcher) fetchOld(path string) (string, error) {
	switch f.mode {
	case ModeShow:
		// Old = parent commit
		return f.git.GetFileContent(f.ref1+"^", path)
	case ModeDiffUnstaged:
		// Old = index (staged content)
		return f.git.GetFileContent("", path)
	case ModeDiffCached:
		// Old = HEAD
		return f.git.GetFileContent("HEAD", path)
	case ModeDiffRefs:
		// Old = first ref
		return f.git.GetFileContent(f.ref1, path)
	default:
		return f.git.GetFileContent("HEAD", path)
	}
}

func (f *Fetcher) fetchNew(path string) (string, error) {
	switch f.mode {
	case ModeShow:
		// New = commit
		return f.git.GetFileContent(f.ref1, path)
	case ModeDiffUnstaged:
		// New = working tree (read file from disk)
		return f.readWorkingTreeFile(path)
	case ModeDiffCached:
		// New = index (staged content)
		return f.git.GetFileContent("", path)
	case ModeDiffRefs:
		// New = second ref
		return f.git.GetFileContent(f.ref2, path)
	default:
		return f.git.GetFileContent("HEAD", path)
	}
}

func (f *Fetcher) readWorkingTreeFile(path string) (string, error) {
	fullPath := path
	if f.WorkDir != "" {
		fullPath = filepath.Join(f.WorkDir, path)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// isFileNotFoundError checks if an error indicates a file was not found.
func isFileNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// Git errors for missing files
	return strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "path") && strings.Contains(msg, "exist") ||
		os.IsNotExist(err)
}
