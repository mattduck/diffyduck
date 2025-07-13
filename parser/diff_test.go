package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiffParser_Parse(t *testing.T) {
	tests := []struct {
		name          string
		diffContent   string
		expectedFiles []FileDiff
		expectedError bool
	}{
		{
			name: "simple file modification",
			diffContent: `diff --git a/test.go b/test.go
index 1234567..abcdefg 100644
--- a/test.go
+++ b/test.go
@@ -1,3 +1,3 @@
 func main() {
-	fmt.Println("hello")
+	fmt.Println("world")
 }`,
			expectedFiles: []FileDiff{
				{
					OldPath: "a/test.go",
					NewPath: "test.go",
					Hunks: []Hunk{
						{
							OldStart: 1,
							OldCount: 3,
							NewStart: 1,
							NewCount: 3,
							Lines: []string{
								" func main() {",
								`-	fmt.Println("hello")`,
								`+	fmt.Println("world")`,
								" }",
							},
						},
					},
				},
			},
			expectedError: false,
		},
		{
			name: "new file creation",
			diffContent: `diff --git a/newfile.txt b/newfile.txt
new file mode 100644
index 0000000..ce01362
--- /dev/null
+++ b/newfile.txt
@@ -0,0 +1,2 @@
+hello
+world`,
			expectedFiles: []FileDiff{
				{
					OldPath: "/dev/null",
					NewPath: "newfile.txt",
					Hunks: []Hunk{
						{
							OldStart: 0,
							OldCount: 0,
							NewStart: 1,
							NewCount: 2,
							Lines: []string{
								"+hello",
								"+world",
							},
						},
					},
				},
			},
			expectedError: false,
		},
		{
			name: "file deletion",
			diffContent: `diff --git a/oldfile.txt b/oldfile.txt
deleted file mode 100644
index ce01362..0000000
--- a/oldfile.txt
+++ /dev/null
@@ -1,2 +0,0 @@
-hello
-world`,
			expectedFiles: []FileDiff{
				{
					OldPath: "a/oldfile.txt",
					NewPath: "/dev/null",
					Hunks: []Hunk{
						{
							OldStart: 1,
							OldCount: 2,
							NewStart: 0,
							NewCount: 0,
							Lines: []string{
								"-hello",
								"-world",
							},
						},
					},
				},
			},
			expectedError: false,
		},
		{
			name: "multiple files",
			diffContent: `diff --git a/file1.txt b/file1.txt
index 1234567..abcdefg 100644
--- a/file1.txt
+++ b/file1.txt
@@ -1 +1 @@
-old content
+new content
diff --git a/file2.txt b/file2.txt
index 9876543..fedcba9 100644
--- a/file2.txt
+++ b/file2.txt
@@ -1,2 +1,2 @@
 line1
-line2
+modified line2`,
			expectedFiles: []FileDiff{
				{
					OldPath: "a/file1.txt",
					NewPath: "file1.txt",
					Hunks: []Hunk{
						{
							OldStart: 1,
							OldCount: 1,
							NewStart: 1,
							NewCount: 1,
							Lines: []string{
								"-old content",
								"+new content",
							},
						},
					},
				},
				{
					OldPath: "a/file2.txt",
					NewPath: "file2.txt",
					Hunks: []Hunk{
						{
							OldStart: 1,
							OldCount: 2,
							NewStart: 1,
							NewCount: 2,
							Lines: []string{
								" line1",
								"-line2",
								"+modified line2",
							},
						},
					},
				},
			},
			expectedError: false,
		},
		{
			name: "multiple hunks in one file",
			diffContent: `diff --git a/test.go b/test.go
index 1234567..abcdefg 100644
--- a/test.go
+++ b/test.go
@@ -1,3 +1,3 @@
 func main() {
-	fmt.Println("hello")
+	fmt.Println("world")
 }
@@ -10,2 +10,2 @@
 func other() {
-	return "old"
+	return "new"
 }`,
			expectedFiles: []FileDiff{
				{
					OldPath: "a/test.go",
					NewPath: "test.go",
					Hunks: []Hunk{
						{
							OldStart: 1,
							OldCount: 3,
							NewStart: 1,
							NewCount: 3,
							Lines: []string{
								" func main() {",
								`-	fmt.Println("hello")`,
								`+	fmt.Println("world")`,
								" }",
							},
						},
						{
							OldStart: 10,
							OldCount: 2,
							NewStart: 10,
							NewCount: 2,
							Lines: []string{
								" func other() {",
								`-	return "old"`,
								`+	return "new"`,
								" }",
							},
						},
					},
				},
			},
			expectedError: false,
		},
		{
			name: "hunk with single line count (no comma)",
			diffContent: `diff --git a/test.txt b/test.txt
index 1234567..abcdefg 100644
--- a/test.txt
+++ b/test.txt
@@ -5 +5 @@
-old line
+new line`,
			expectedFiles: []FileDiff{
				{
					OldPath: "a/test.txt",
					NewPath: "test.txt",
					Hunks: []Hunk{
						{
							OldStart: 5,
							OldCount: 1,
							NewStart: 5,
							NewCount: 1,
							Lines: []string{
								"-old line",
								"+new line",
							},
						},
					},
				},
			},
			expectedError: false,
		},
		{
			name:          "empty diff",
			diffContent:   "",
			expectedFiles: nil,
			expectedError: false,
		},
		{
			name: "diff with b/ prefix in new file path",
			diffContent: `diff --git a/test.go b/test.go
index 1234567..abcdefg 100644
--- a/test.go
+++ b/test.go
@@ -1 +1 @@
-old
+new`,
			expectedFiles: []FileDiff{
				{
					OldPath: "a/test.go",
					NewPath: "test.go",
					Hunks: []Hunk{
						{
							OldStart: 1,
							OldCount: 1,
							NewStart: 1,
							NewCount: 1,
							Lines: []string{
								"-old",
								"+new",
							},
						},
					},
				},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parser := NewDiffParser()
			result, err := parser.Parse(tt.diffContent)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedFiles, result)
		})
	}
}

func TestNewDiffParser(t *testing.T) {
	parser := NewDiffParser()

	assert.NotNil(t, parser)
	assert.NotNil(t, parser.fileHeaderRe)
	assert.NotNil(t, parser.hunkHeaderRe)
	assert.NotNil(t, parser.oldFileRe)
	assert.NotNil(t, parser.newFileRe)
}

func TestDiffParser_RegexPatterns(t *testing.T) {
	parser := NewDiffParser()

	tests := []struct {
		name    string
		regex   string
		input   string
		matches []string
	}{
		{
			name:    "file header regex",
			regex:   "fileHeaderRe",
			input:   "diff --git a/old.txt b/new.txt",
			matches: []string{"diff --git a/old.txt b/new.txt", "old.txt", "new.txt"},
		},
		{
			name:    "hunk header regex with counts",
			regex:   "hunkHeaderRe",
			input:   "@@ -1,5 +1,7 @@",
			matches: []string{"@@ -1,5 +1,7 @@", "1", "5", "1", "7"},
		},
		{
			name:    "hunk header regex without counts",
			regex:   "hunkHeaderRe",
			input:   "@@ -1 +1 @@",
			matches: []string{"@@ -1 +1 @@", "1", "", "1", ""},
		},
		{
			name:    "old file regex",
			regex:   "oldFileRe",
			input:   "--- a/test.txt",
			matches: []string{"--- a/test.txt", "a/test.txt"},
		},
		{
			name:    "new file regex",
			regex:   "newFileRe",
			input:   "+++ b/test.txt",
			matches: []string{"+++ b/test.txt", "b/test.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var matches []string
			switch tt.regex {
			case "fileHeaderRe":
				matches = parser.fileHeaderRe.FindStringSubmatch(tt.input)
			case "hunkHeaderRe":
				matches = parser.hunkHeaderRe.FindStringSubmatch(tt.input)
			case "oldFileRe":
				matches = parser.oldFileRe.FindStringSubmatch(tt.input)
			case "newFileRe":
				matches = parser.newFileRe.FindStringSubmatch(tt.input)
			}

			assert.Equal(t, tt.matches, matches)
		})
	}
}

// Benchmark tests for performance measurement
func BenchmarkDiffParser_Parse(b *testing.B) {
	diffContent := `diff --git a/test.go b/test.go
index 1234567..abcdefg 100644
--- a/test.go
+++ b/test.go
@@ -1,3 +1,3 @@
 func main() {
-	fmt.Println("hello")
+	fmt.Println("world")
 }`

	parser := NewDiffParser()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.Parse(diffContent)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDiffParser_ParseLarge(b *testing.B) {
	// Create a large diff with multiple files and hunks
	largeDiff := ""
	for i := 0; i < 100; i++ {
		largeDiff += `diff --git a/file` + string(rune(i)) + `.txt b/file` + string(rune(i)) + `.txt
index 1234567..abcdefg 100644
--- a/file` + string(rune(i)) + `.txt
+++ b/file` + string(rune(i)) + `.txt
@@ -1,3 +1,3 @@
 line1
-old line ` + string(rune(i)) + `
+new line ` + string(rune(i)) + `
 line3
`
	}

	parser := NewDiffParser()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.Parse(largeDiff)
		if err != nil {
			b.Fatal(err)
		}
	}
}
