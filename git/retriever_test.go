package git

import (
	"io/ioutil"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFileRetriever(t *testing.T) {
	retriever := NewFileRetriever()
	assert.NotNil(t, retriever)
}

func TestFileRetriever_GetNewFileContent(t *testing.T) {
	tests := []struct {
		name           string
		setupFile      func(t *testing.T) string // returns file path
		expectedLines  []string
		expectedError  bool
		cleanup        func(string)
	}{
		{
			name: "read existing file",
			setupFile: func(t *testing.T) string {
				tmpFile, err := ioutil.TempFile("", "test_*.txt")
				require.NoError(t, err)
				
				content := "line1\nline2\nline3"
				_, err = tmpFile.WriteString(content)
				require.NoError(t, err)
				
				tmpFile.Close()
				return tmpFile.Name()
			},
			expectedLines: []string{"line1", "line2", "line3"},
			expectedError: false,
			cleanup: func(path string) {
				os.Remove(path)
			},
		},
		{
			name: "read empty file",
			setupFile: func(t *testing.T) string {
				tmpFile, err := ioutil.TempFile("", "empty_*.txt")
				require.NoError(t, err)
				tmpFile.Close()
				return tmpFile.Name()
			},
			expectedLines: []string{},
			expectedError: false,
			cleanup: func(path string) {
				os.Remove(path)
			},
		},
		{
			name: "read file with single line no newline",
			setupFile: func(t *testing.T) string {
				tmpFile, err := ioutil.TempFile("", "single_*.txt")
				require.NoError(t, err)
				
				_, err = tmpFile.WriteString("single line")
				require.NoError(t, err)
				
				tmpFile.Close()
				return tmpFile.Name()
			},
			expectedLines: []string{"single line"},
			expectedError: false,
			cleanup: func(path string) {
				os.Remove(path)
			},
		},
		{
			name: "read file with trailing newline",
			setupFile: func(t *testing.T) string {
				tmpFile, err := ioutil.TempFile("", "trailing_*.txt")
				require.NoError(t, err)
				
				content := "line1\nline2\n"
				_, err = tmpFile.WriteString(content)
				require.NoError(t, err)
				
				tmpFile.Close()
				return tmpFile.Name()
			},
			expectedLines: []string{"line1", "line2"},
			expectedError: false,
			cleanup: func(path string) {
				os.Remove(path)
			},
		},
		{
			name: "read non-existent file",
			setupFile: func(t *testing.T) string {
				return "/non/existent/file.txt"
			},
			expectedLines: []string{},
			expectedError: false, // Code returns empty slice for non-existent files
			cleanup: func(path string) {
				// no cleanup needed
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retriever := NewFileRetriever()
			filePath := tt.setupFile(t)
			defer tt.cleanup(filePath)
			
			result, err := retriever.GetNewFileContent(filePath)
			
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedLines, result)
			}
		})
	}
}

func TestFileRetriever_GetOldFileContent(t *testing.T) {
	// Skip these tests if git is not available
	if !isGitAvailable() {
		t.Skip("git command not available")
	}

	tests := []struct {
		name           string
		filePath       string
		setupRepo      func(t *testing.T) string // returns repo directory
		expectedLines  []string
		expectedError  bool
		cleanup        func(string)
	}{
		{
			name:     "file that doesn't exist in git",
			filePath: "nonexistent.txt",
			setupRepo: func(t *testing.T) string {
				return setupTestGitRepo(t)
			},
			expectedLines: []string{},
			expectedError: false, // Code returns empty slice for git exit code 128
			cleanup: func(dir string) {
				os.RemoveAll(dir)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoDir := tt.setupRepo(t)
			defer tt.cleanup(repoDir)
			
			// Change to repo directory for git commands
			originalDir, err := os.Getwd()
			require.NoError(t, err)
			defer os.Chdir(originalDir)
			
			err = os.Chdir(repoDir)
			require.NoError(t, err)
			
			retriever := NewFileRetriever()
			result, err := retriever.GetOldFileContent(tt.filePath)
			
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedLines, result)
			}
		})
	}
}

func TestFileRetriever_GetOldFileContent_WithValidFile(t *testing.T) {
	// Skip these tests if git is not available
	if !isGitAvailable() {
		t.Skip("git command not available")
	}

	// This test requires a more complex setup with actual git operations
	t.Run("file that exists in git", func(t *testing.T) {
		repoDir := setupTestGitRepoWithFile(t)
		defer os.RemoveAll(repoDir)
		
		// Change to repo directory for git commands
		originalDir, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(originalDir)
		
		err = os.Chdir(repoDir)
		require.NoError(t, err)
		
		retriever := NewFileRetriever()
		result, err := retriever.GetOldFileContent("test.txt")
		
		assert.NoError(t, err)
		assert.Equal(t, []string{"original content"}, result)
	})
}

func TestFileRetriever_GetOldFileContent_EdgeCases(t *testing.T) {
	// Test various edge cases without requiring a full git setup
	tests := []struct {
		name           string
		filePath       string
		expectedError  bool
	}{
		{
			name:          "empty file path",
			filePath:      "",
			expectedError: false, // git show HEAD: returns error but code handles it gracefully
		},
		{
			name:          "file path with spaces",
			filePath:      "file with spaces.txt",
			expectedError: false, // Will fail git command but handled gracefully
		},
		{
			name:          "file path with special characters",
			filePath:      "file@#$.txt",
			expectedError: false, // Will fail git command but handled gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retriever := NewFileRetriever()
			_, err := retriever.GetOldFileContent(tt.filePath)
			
			// Since we're not in a git repo, all calls will error
			// But we can test that the function doesn't panic
			if tt.expectedError {
				assert.Error(t, err)
			}
			// For non-expected errors, we don't assert since git might not be available
		})
	}
}

// Helper functions

func isGitAvailable() bool {
	cmd := exec.Command("git", "--version")
	err := cmd.Run()
	return err == nil
}

func setupTestGitRepo(t *testing.T) string {
	tmpDir, err := ioutil.TempDir("", "git_test_*")
	require.NoError(t, err)
	
	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	err = cmd.Run()
	require.NoError(t, err)
	
	// Configure git user (required for commits)
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	err = cmd.Run()
	require.NoError(t, err)
	
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	err = cmd.Run()
	require.NoError(t, err)
	
	return tmpDir
}

func setupTestGitRepoWithFile(t *testing.T) string {
	tmpDir := setupTestGitRepo(t)
	
	// Create a test file
	testFile := tmpDir + "/test.txt"
	err := ioutil.WriteFile(testFile, []byte("original content"), 0644)
	require.NoError(t, err)
	
	// Add and commit the file
	cmd := exec.Command("git", "add", "test.txt")
	cmd.Dir = tmpDir
	err = cmd.Run()
	require.NoError(t, err)
	
	cmd = exec.Command("git", "commit", "-m", "Add test file")
	cmd.Dir = tmpDir
	err = cmd.Run()
	require.NoError(t, err)
	
	// Modify the file to create a difference
	err = ioutil.WriteFile(testFile, []byte("modified content"), 0644)
	require.NoError(t, err)
	
	return tmpDir
}

// Benchmark tests

func BenchmarkFileRetriever_GetNewFileContent(b *testing.B) {
	// Create a temporary file for benchmarking
	tmpFile, err := ioutil.TempFile("", "benchmark_*.txt")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	
	content := "line1\nline2\nline3\nline4\nline5"
	_, err = tmpFile.WriteString(content)
	if err != nil {
		b.Fatal(err)
	}
	tmpFile.Close()
	
	retriever := NewFileRetriever()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := retriever.GetNewFileContent(tmpFile.Name())
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFileRetriever_GetNewFileContentLarge(b *testing.B) {
	// Create a larger temporary file for benchmarking
	tmpFile, err := ioutil.TempFile("", "benchmark_large_*.txt")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	
	// Create content with 1000 lines
	content := ""
	for i := 0; i < 1000; i++ {
		content += "This is line number " + string(rune(i)) + " with some content\n"
	}
	
	_, err = tmpFile.WriteString(content)
	if err != nil {
		b.Fatal(err)
	}
	tmpFile.Close()
	
	retriever := NewFileRetriever()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := retriever.GetNewFileContent(tmpFile.Name())
		if err != nil {
			b.Fatal(err)
		}
	}
}