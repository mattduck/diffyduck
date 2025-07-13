package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

type FileType int

const (
	TextFile FileType = iota
	BinaryFile
	DeletedFile
	NewFile
)

type FileInfo struct {
	Type  FileType
	Lines []string
}

type FileRetriever struct{}

func NewFileRetriever() *FileRetriever {
	return &FileRetriever{}
}

func (r *FileRetriever) GetOldFileContent(filePath string) ([]string, error) {
	cmd := exec.Command("git", "show", fmt.Sprintf("HEAD:%s", filePath))
	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 128 {
				return []string{}, nil
			}
		}
		return nil, fmt.Errorf("failed to get old file content for %s: %v", filePath, err)
	}

	content := string(output)
	if content == "" {
		return []string{}, nil
	}

	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	return lines, nil
}

func (r *FileRetriever) GetNewFileContent(filePath string) ([]string, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return []string{}, nil
	}

	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read new file content for %s: %v", filePath, err)
	}

	contentStr := string(content)
	if contentStr == "" {
		return []string{}, nil
	}

	lines := strings.Split(strings.TrimSuffix(contentStr, "\n"), "\n")
	return lines, nil
}

func (r *FileRetriever) GetFileInfo(filePath string, isOld bool) (*FileInfo, error) {
	// Check if file is deleted
	if filePath == "/dev/null" {
		if isOld {
			return &FileInfo{Type: DeletedFile, Lines: []string{}}, nil
		} else {
			return &FileInfo{Type: NewFile, Lines: []string{}}, nil
		}
	}

	// Get file content
	var content []byte
	var err error

	if isOld {
		cmd := exec.Command("git", "show", fmt.Sprintf("HEAD:%s", filePath))
		content, err = cmd.Output()
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				if exitError.ExitCode() == 128 {
					return &FileInfo{Type: NewFile, Lines: []string{}}, nil
				}
			}
			return nil, fmt.Errorf("failed to get old file content for %s: %v", filePath, err)
		}
	} else {
		if _, statErr := os.Stat(filePath); os.IsNotExist(statErr) {
			return &FileInfo{Type: DeletedFile, Lines: []string{}}, nil
		}

		content, err = ioutil.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read new file content for %s: %v", filePath, err)
		}
	}

	// Check if binary
	if isBinary(content) {
		return &FileInfo{Type: BinaryFile, Lines: []string{}}, nil
	}

	// Convert to lines
	contentStr := string(content)
	if contentStr == "" {
		return &FileInfo{Type: TextFile, Lines: []string{}}, nil
	}

	lines := strings.Split(strings.TrimSuffix(contentStr, "\n"), "\n")
	return &FileInfo{Type: TextFile, Lines: lines}, nil
}

func isBinary(content []byte) bool {
	// Check for null bytes (common indicator of binary content)
	for _, b := range content {
		if b == 0 {
			return true
		}
	}
	return false
}
