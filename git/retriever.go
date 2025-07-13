package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

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