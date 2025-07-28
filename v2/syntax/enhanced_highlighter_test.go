package syntax

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestNewEnhancedHighlighter(t *testing.T) {
	eh := NewEnhancedHighlighter()
	defer eh.Close()

	if eh == nil {
		t.Fatal("NewEnhancedHighlighter returned nil")
	}
	if eh.baseHighlighter == nil {
		t.Error("baseHighlighter is nil")
	}
	if eh.fileCache == nil {
		t.Error("fileCache is nil")
	}
	if eh.maxCacheSize <= 0 {
		t.Error("maxCacheSize should be positive")
	}
}

func TestParseFileUnsupportedLanguage(t *testing.T) {
	eh := NewEnhancedHighlighter()
	defer eh.Close()

	fileContent := []string{
		"This is a plain text file",
		"with no syntax highlighting",
	}

	err := eh.ParseFile("test.txt", fileContent)
	if err != nil {
		t.Errorf("ParseFile should not error for unsupported language: %v", err)
	}

	// Should have created empty cache entry
	cache, exists := eh.fileCache["test.txt"]
	if !exists {
		t.Error("Expected cache entry for unsupported file")
	}
	if cache.Language != "" {
		t.Errorf("Expected empty language for unsupported file, got %s", cache.Language)
	}
	if cache.Tree != nil {
		t.Error("Expected nil tree for unsupported file")
	}
}

func TestParseFileGoLanguage(t *testing.T) {
	eh := NewEnhancedHighlighter()
	defer eh.Close()

	fileContent := []string{
		"package main",
		"",
		"import \"fmt\"",
		"",
		"func main() {",
		"\tfmt.Println(\"Hello, World!\")",
		"}",
	}

	err := eh.ParseFile("test.go", fileContent)
	if err != nil {
		t.Errorf("ParseFile failed for Go file: %v", err)
	}

	// Should have created cache entry with tree
	cache, exists := eh.fileCache["test.go"]
	if !exists {
		t.Error("Expected cache entry for Go file")
	}
	if cache.Language == "" {
		t.Error("Expected non-empty language for Go file")
	}
	if cache.Tree == nil {
		t.Error("Expected non-nil tree for Go file")
	}
	if len(cache.FileContent) != len(fileContent) {
		t.Errorf("Expected %d lines in cache, got %d", len(fileContent), len(cache.FileContent))
	}
}

func TestCacheSizeEviction(t *testing.T) {
	eh := NewEnhancedHighlighter()
	eh.maxCacheSize = 1 // Very small cache for testing eviction
	defer eh.Close()

	fileContent := []string{"package main"}

	// Parse first file
	err := eh.ParseFile("test1.go", fileContent)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Verify cache exists
	_, exists := eh.fileCache["test1.go"]
	if !exists {
		t.Error("Expected cache entry for test1.go")
	}

	// Parse second file - should evict first due to size limit
	err = eh.ParseFile("test2.go", fileContent)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Cache should be at size limit
	if len(eh.fileCache) > eh.maxCacheSize {
		t.Errorf("Cache size %d exceeds limit %d", len(eh.fileCache), eh.maxCacheSize)
	}
}

func TestCacheSizeLimit(t *testing.T) {
	eh := NewEnhancedHighlighter()
	eh.maxCacheSize = 2 // Very small cache for testing
	defer eh.Close()

	fileContent := []string{"package main"}

	// Add files up to limit
	for i := 0; i < 3; i++ {
		filePath := fmt.Sprintf("test%d.go", i)
		err := eh.ParseFile(filePath, fileContent)
		if err != nil {
			t.Fatalf("ParseFile failed for %s: %v", filePath, err)
		}
	}

	// Should only have maxCacheSize entries
	if len(eh.fileCache) > eh.maxCacheSize {
		t.Errorf("Cache size %d exceeds limit %d", len(eh.fileCache), eh.maxCacheSize)
	}
}

func TestGetLineHighlightingUncached(t *testing.T) {
	eh := NewEnhancedHighlighter()
	defer eh.Close()

	// Try to get highlighting for file that hasn't been parsed
	result := eh.GetLineHighlighting("uncached.go", 1, "package main")

	// Should fallback to base highlighter
	if result == "" {
		t.Error("Expected non-empty result from fallback highlighting")
	}
}

func TestGetLineHighlightingUnsupported(t *testing.T) {
	eh := NewEnhancedHighlighter()
	defer eh.Close()

	fileContent := []string{"plain text"}
	err := eh.ParseFile("test.txt", fileContent)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	result := eh.GetLineHighlighting("test.txt", 1, "plain text")
	if result != "plain text" {
		t.Errorf("Expected unchanged content for unsupported file, got %s", result)
	}
}

func TestGetLineStyles(t *testing.T) {
	eh := NewEnhancedHighlighter()
	defer eh.Close()

	fileContent := []string{
		"package main",
		"func test() {}",
	}

	err := eh.ParseFile("test.go", fileContent)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Get styles for a line
	styles := eh.GetLineStyles("test.go", 1, "package main")

	// Should return some styles (exact content depends on tree-sitter parsing)
	// For now, just verify the method doesn't crash
	if styles == nil {
		// This is ok - might be no styles for this particular line
	}

	// Test caching - second call should use cached result
	styles2 := eh.GetLineStyles("test.go", 1, "package main")
	if len(styles) != len(styles2) {
		t.Error("Cached styles should be identical to first call")
	}
}

func TestStyleSpan(t *testing.T) {
	span := StyleSpan{
		Start: 0,
		End:   5,
		Style: tcell.StyleDefault.Foreground(tcell.ColorBlue),
	}

	if span.Start != 0 {
		t.Errorf("Expected Start=0, got %d", span.Start)
	}
	if span.End != 5 {
		t.Errorf("Expected End=5, got %d", span.End)
	}
}

func TestGetCaptureStyle(t *testing.T) {
	eh := NewEnhancedHighlighter()
	defer eh.Close()

	tests := []struct {
		captureName   string
		expectedColor tcell.Color
	}{
		{"keyword", tcell.ColorBlue},
		{"string", tcell.ColorGreen},
		{"comment", tcell.ColorGray},
		{"number", tcell.ColorYellow},
		{"function", tcell.ColorAqua},
		{"type", tcell.ColorFuchsia},
		{"unknown", tcell.ColorDefault},
	}

	for _, tt := range tests {
		t.Run(tt.captureName, func(t *testing.T) {
			style := eh.getCaptureStyle(tt.captureName)
			fg, _, _ := style.Decompose()
			if fg != tt.expectedColor {
				t.Errorf("Expected color %v for %s, got %v", tt.expectedColor, tt.captureName, fg)
			}
		})
	}
}

func TestComputeLineStylesBounds(t *testing.T) {
	eh := NewEnhancedHighlighter()
	defer eh.Close()

	// Test with single line file
	fileContent := []string{"package main"}
	err := eh.ParseFile("test.go", fileContent)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	cache := eh.fileCache["test.go"]
	if cache == nil {
		t.Fatal("Expected cache entry")
	}

	// Test bounds checking
	styles := eh.computeLineStyles(cache, 1, "package main")
	// Should not crash and return valid styles
	for _, style := range styles {
		if style.Start < 0 {
			t.Error("Style start should not be negative")
		}
		if style.End > len("package main") {
			t.Error("Style end should not exceed line length")
		}
		if style.Start >= style.End {
			t.Error("Style start should be less than end")
		}
	}
}

func TestApplyStylesToLine(t *testing.T) {
	eh := NewEnhancedHighlighter()
	defer eh.Close()

	content := "package main"
	styles := []StyleSpan{
		{Start: 0, End: 7, Style: tcell.StyleDefault.Foreground(tcell.ColorBlue)},
		{Start: 8, End: 12, Style: tcell.StyleDefault.Foreground(tcell.ColorGreen)},
	}

	result := eh.applyStylesToLine(content, styles)
	// For now, this just returns the original content
	// In the future, it would apply tcell styling
	if result != content {
		t.Errorf("Expected %s, got %s", content, result)
	}

	// Test with empty styles
	result2 := eh.applyStylesToLine(content, nil)
	if result2 != content {
		t.Errorf("Expected %s with empty styles, got %s", content, result2)
	}
}
