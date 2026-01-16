package tui

// FileContentLoadedMsg is sent when file content has been fetched for a single file.
type FileContentLoadedMsg struct {
	FileIndex  int
	OldContent []string
	NewContent []string
	Err        error
}

// AllContentLoadedMsg is sent when content for all files has been fetched.
type AllContentLoadedMsg struct {
	Contents []FileContent
}

// FileContent holds the fetched content for a single file.
type FileContent struct {
	FileIndex  int
	OldContent []string
	NewContent []string
	Err        error
}
