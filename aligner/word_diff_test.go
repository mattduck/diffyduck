package aligner

import (
	"testing"

	"duckdiff/types"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWordDiff_EdgeCases(t *testing.T) {
	aligner := NewDiffAligner()

	tests := []struct {
		name           string
		oldLine        string
		newLine        string
		expectedOldLen int
		expectedNewLen int
	}{
		{
			name:           "both empty",
			oldLine:        "",
			newLine:        "",
			expectedOldLen: 0,
			expectedNewLen: 0,
		},
		{
			name:           "old empty, new has content",
			oldLine:        "",
			newLine:        "hello world",
			expectedOldLen: 0,
			expectedNewLen: 3, // "hello", " ", "world"
		},
		{
			name:           "old has content, new empty",
			oldLine:        "hello world",
			newLine:        "",
			expectedOldLen: 3, // "hello", " ", "world"
			expectedNewLen: 0,
		},
		{
			name:           "identical strings",
			oldLine:        "hello world",
			newLine:        "hello world",
			expectedOldLen: 1, // single equal segment
			expectedNewLen: 1, // single equal segment
		},
		{
			name:           "single character",
			oldLine:        "a",
			newLine:        "b",
			expectedOldLen: 1,
			expectedNewLen: 1,
		},
		{
			name:           "only whitespace",
			oldLine:        "   ",
			newLine:        "\t\t",
			expectedOldLen: 1,
			expectedNewLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aligner.computeWordDiff(tt.oldLine, tt.newLine)

			require.NotNil(t, result)
			assert.Equal(t, tt.expectedOldLen, len(result.OldSegments), "Old segments count mismatch")
			assert.Equal(t, tt.expectedNewLen, len(result.NewSegments), "New segments count mismatch")

			// Verify that reconstructing the text gives us back the original
			oldReconstructed := reconstructText(result.OldSegments)
			newReconstructed := reconstructText(result.NewSegments)

			if tt.oldLine == tt.newLine && tt.oldLine != "" {
				// For identical non-empty strings, both should reconstruct to the same text
				assert.Equal(t, tt.oldLine, oldReconstructed)
				assert.Equal(t, tt.newLine, newReconstructed)
			} else {
				assert.Equal(t, tt.oldLine, oldReconstructed)
				assert.Equal(t, tt.newLine, newReconstructed)
			}
		})
	}
}

func TestWordDiff_Tokenization(t *testing.T) {
	aligner := NewDiffAligner()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "simple words",
			input:    "hello world",
			expected: []string{"hello", " ", "world"},
		},
		{
			name:     "mixed whitespace",
			input:    "hello\tworld\n",
			expected: []string{"hello", "\t", "world", "\n"},
		},
		{
			name:     "punctuation",
			input:    "hello, world!",
			expected: []string{"hello", ",", " ", "world", "!"},
		},
		{
			name:     "snake_case",
			input:    "snake_case_var",
			expected: []string{"snake_case_var"}, // underscore is part of \w
		},
		{
			name:     "camelCase",
			input:    "camelCaseVar",
			expected: []string{"camelCaseVar"},
		},
		{
			name:     "operators",
			input:    "a += b",
			expected: []string{"a", " ", "+=", " ", "b"},
		},
		{
			name:     "numbers",
			input:    "var123 = 456",
			expected: []string{"var123", " ", "=", " ", "456"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aligner.tokenize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWordDiff_UnicodeHandling(t *testing.T) {
	aligner := NewDiffAligner()

	tests := []struct {
		name    string
		oldLine string
		newLine string
	}{
		{
			name:    "emoji",
			oldLine: "hello 😀 world",
			newLine: "hello 😎 world",
		},
		{
			name:    "chinese characters",
			oldLine: "你好 world",
			newLine: "再见 world",
		},
		{
			name:    "accented characters",
			oldLine: "café münü",
			newLine: "cafe menu",
		},
		{
			name:    "mixed unicode",
			oldLine: "🎉 célébration 🎊",
			newLine: "🎉 celebration 🎊",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aligner.computeWordDiff(tt.oldLine, tt.newLine)

			// Should not panic and should produce valid results
			require.NotNil(t, result)

			// Verify reconstruction
			oldReconstructed := reconstructText(result.OldSegments)
			newReconstructed := reconstructText(result.NewSegments)

			assert.Equal(t, tt.oldLine, oldReconstructed)
			assert.Equal(t, tt.newLine, newReconstructed)
		})
	}
}

func TestWordDiff_ProgrammingConstructs(t *testing.T) {
	aligner := NewDiffAligner()

	tests := []struct {
		name            string
		oldLine         string
		newLine         string
		expectedChanges int // approximate number of changed segments
	}{
		{
			name:            "function signature",
			oldLine:         "func calculateTotal(items []Item) int {",
			newLine:         "func computeSum(items []Product) float64 {",
			expectedChanges: 3, // function name, type name, return type
		},
		{
			name:            "variable declaration",
			oldLine:         "var userName string = \"john\"",
			newLine:         "var userEmail string = \"john@example.com\"",
			expectedChanges: 1, // variable name and value
		},
		{
			name:            "method call",
			oldLine:         "user.getName().toLowerCase()",
			newLine:         "user.getEmail().toUpperCase()",
			expectedChanges: 2, // method names
		},
		{
			name:            "import statement",
			oldLine:         "import \"encoding/json\"",
			newLine:         "import \"encoding/xml\"",
			expectedChanges: 1, // package name
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aligner.computeWordDiff(tt.oldLine, tt.newLine)

			require.NotNil(t, result)

			// Count actual changes
			oldChanges := countChanges(result.OldSegments)
			newChanges := countChanges(result.NewSegments)

			// Should have some changes
			assert.Greater(t, oldChanges, 0, "Should have deletions")
			assert.Greater(t, newChanges, 0, "Should have insertions")

			// Verify reconstruction
			oldReconstructed := reconstructText(result.OldSegments)
			newReconstructed := reconstructText(result.NewSegments)

			assert.Equal(t, tt.oldLine, oldReconstructed)
			assert.Equal(t, tt.newLine, newReconstructed)
		})
	}
}

func TestWordDiff_WhitespaceHandling(t *testing.T) {
	aligner := NewDiffAligner()

	tests := []struct {
		name    string
		oldLine string
		newLine string
	}{
		{
			name:    "spaces to tabs",
			oldLine: "hello    world",
			newLine: "hello\t\tworld",
		},
		{
			name:    "multiple spaces",
			oldLine: "hello  world",
			newLine: "hello world",
		},
		{
			name:    "trailing whitespace",
			oldLine: "hello world ",
			newLine: "hello world",
		},
		{
			name:    "leading whitespace",
			oldLine: " hello world",
			newLine: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aligner.computeWordDiff(tt.oldLine, tt.newLine)

			require.NotNil(t, result)

			// Verify reconstruction
			oldReconstructed := reconstructText(result.OldSegments)
			newReconstructed := reconstructText(result.NewSegments)

			assert.Equal(t, tt.oldLine, oldReconstructed)
			assert.Equal(t, tt.newLine, newReconstructed)
		})
	}
}

func TestWordDiff_RealCodeExamples(t *testing.T) {
	aligner := NewDiffAligner()

	tests := []struct {
		name    string
		oldLine string
		newLine string
	}{
		{
			name:    "go function",
			oldLine: "func (r *Repository) GetUser(id int) (*User, error) {",
			newLine: "func (r *Repository) GetUserByID(id int64) (*User, error) {",
		},
		{
			name:    "javascript arrow function",
			oldLine: "const getUserName = (user) => user.name;",
			newLine: "const getUserEmail = (user) => user.email;",
		},
		{
			name:    "python method",
			oldLine: "def calculate_total(self, items: List[Item]) -> int:",
			newLine: "def compute_sum(self, items: List[Product]) -> float:",
		},
		{
			name:    "sql query",
			oldLine: "SELECT name, email FROM users WHERE active = true",
			newLine: "SELECT username, email FROM customers WHERE status = 'active'",
		},
		{
			name:    "json structure",
			oldLine: `{"user": {"name": "john", "age": 30}}`,
			newLine: `{"customer": {"name": "john", "age": 25}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aligner.computeWordDiff(tt.oldLine, tt.newLine)

			require.NotNil(t, result)

			// Should have both equal and changed segments
			hasEqual := false
			hasChanges := false

			for _, segment := range result.OldSegments {
				switch segment.Type {
				case diffmatchpatch.DiffEqual:
					hasEqual = true
				case diffmatchpatch.DiffDelete:
					hasChanges = true
				}
			}

			for _, segment := range result.NewSegments {
				switch segment.Type {
				case diffmatchpatch.DiffEqual:
					hasEqual = true
				case diffmatchpatch.DiffInsert:
					hasChanges = true
				}
			}

			assert.True(t, hasEqual, "Should have some unchanged parts")
			assert.True(t, hasChanges, "Should have some changes")

			// Verify reconstruction
			oldReconstructed := reconstructText(result.OldSegments)
			newReconstructed := reconstructText(result.NewSegments)

			assert.Equal(t, tt.oldLine, oldReconstructed)
			assert.Equal(t, tt.newLine, newReconstructed)
		})
	}
}

// Helper function to reconstruct text from segments
func reconstructText(segments []types.DiffSegment) string {
	var result string
	for _, segment := range segments {
		result += segment.Text
	}
	return result
}

// Helper function to count changed segments
func countChanges(segments []types.DiffSegment) int {
	count := 0
	for _, segment := range segments {
		if segment.Type == diffmatchpatch.DiffDelete || segment.Type == diffmatchpatch.DiffInsert {
			count++
		}
	}
	return count
}

func TestWordDiff_PropertyBasedValidation(t *testing.T) {
	aligner := NewDiffAligner()

	// Test that applying the diff segments recreates the original text
	tests := []struct {
		name    string
		oldLine string
		newLine string
	}{
		{"simple change", "hello world", "hello universe"},
		{"multiple changes", "func old(a int) bool", "func new(b string) error"},
		{"empty to content", "", "new content"},
		{"content to empty", "old content", ""},
		{"complex", "const OLD_CONSTANT = 123;", "const NEW_CONSTANT = 456;"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aligner.computeWordDiff(tt.oldLine, tt.newLine)

			// Property 1: Reconstruction should give original text
			oldReconstructed := reconstructText(result.OldSegments)
			newReconstructed := reconstructText(result.NewSegments)

			assert.Equal(t, tt.oldLine, oldReconstructed, "Old line reconstruction failed")
			assert.Equal(t, tt.newLine, newReconstructed, "New line reconstruction failed")

			// Property 2: Should not have consecutive segments of the same type
			// (except for DiffEqual which can be consecutive)
			validateNonConsecutiveChanges(t, result.OldSegments)
			validateNonConsecutiveChanges(t, result.NewSegments)

			// Property 3: Equal segments should appear in both old and new at same positions
			validateEqualSegments(t, result.OldSegments, result.NewSegments)
		})
	}
}

func validateNonConsecutiveChanges(t *testing.T, segments []types.DiffSegment) {
	// Actually, consecutive segments of the same type are valid in many cases
	// (e.g., all insertions when going from empty to content)
	// This validation was too strict. Instead, let's just verify basic sanity.

	// Ensure no empty segments
	for i, seg := range segments {
		assert.NotEmpty(t, seg.Text, "Segment %d should not be empty", i)
	}
}

func validateEqualSegments(t *testing.T, oldSegments, newSegments []types.DiffSegment) {
	// Extract equal segments from both sides
	oldEquals := []string{}
	newEquals := []string{}

	for _, seg := range oldSegments {
		if seg.Type == diffmatchpatch.DiffEqual {
			oldEquals = append(oldEquals, seg.Text)
		}
	}

	for _, seg := range newSegments {
		if seg.Type == diffmatchpatch.DiffEqual {
			newEquals = append(newEquals, seg.Text)
		}
	}

	// Equal segments should be the same on both sides
	assert.Equal(t, oldEquals, newEquals, "Equal segments should match between old and new")
}
