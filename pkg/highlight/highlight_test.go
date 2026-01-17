package highlight

import (
	"testing"
)

func TestHighlighter_SimpleGoCode(t *testing.T) {
	h := New()
	defer h.Close()

	code := []byte(`package main

func hello() {
	fmt.Println("Hello, world!")
}
`)

	spans, err := h.Highlight("test.go", code)
	if err != nil {
		t.Fatalf("Highlight failed: %v", err)
	}

	if len(spans) == 0 {
		t.Fatal("Expected spans, got none")
	}

	// Check that we have some expected categories
	categories := make(map[Category]bool)
	for _, s := range spans {
		categories[s.Category] = true
	}

	// Should have keywords (package, func)
	if !categories[CategoryKeyword] {
		t.Error("Expected CategoryKeyword in spans")
	}

	// Should have a string literal
	if !categories[CategoryString] {
		t.Error("Expected CategoryString in spans")
	}

	// Should have a function definition
	if !categories[CategoryFunction] {
		t.Error("Expected CategoryFunction in spans")
	}
}

func TestHighlighter_GoKeywords(t *testing.T) {
	h := New()
	defer h.Close()

	code := []byte(`package test

func foo() {
	if true {
		return
	}
	for i := range items {
		continue
	}
}
`)

	spans, err := h.Highlight("test.go", code)
	if err != nil {
		t.Fatalf("Highlight failed: %v", err)
	}

	// Find all keyword spans
	keywords := []string{}
	for _, s := range spans {
		if s.Category == CategoryKeyword {
			keywords = append(keywords, string(code[s.Start:s.End]))
		}
	}

	expected := []string{"package", "func", "if", "return", "for", "range", "continue"}
	for _, kw := range expected {
		found := false
		for _, got := range keywords {
			if got == kw {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected keyword %q not found in %v", kw, keywords)
		}
	}
}

func TestHighlighter_GoLiterals(t *testing.T) {
	h := New()
	defer h.Close()

	code := []byte(`package test

var (
	s1 = "hello"
	s2 = ` + "`raw`" + `
	n1 = 42
	n2 = 3.14
	b1 = true
	b2 = false
	x  = nil
)
`)

	spans, err := h.Highlight("test.go", code)
	if err != nil {
		t.Fatalf("Highlight failed: %v", err)
	}

	// Check for literal categories
	categories := make(map[Category]bool)
	for _, s := range spans {
		categories[s.Category] = true
	}

	if !categories[CategoryString] {
		t.Error("Expected CategoryString")
	}
	if !categories[CategoryNumber] {
		t.Error("Expected CategoryNumber")
	}
	if !categories[CategoryBoolean] {
		t.Error("Expected CategoryBoolean")
	}
	if !categories[CategoryNil] {
		t.Error("Expected CategoryNil")
	}
}

func TestHighlighter_GoComments(t *testing.T) {
	h := New()
	defer h.Close()

	code := []byte(`package test

// This is a comment
func foo() {
	/* multi
	   line */
}
`)

	spans, err := h.Highlight("test.go", code)
	if err != nil {
		t.Fatalf("Highlight failed: %v", err)
	}

	commentCount := 0
	for _, s := range spans {
		if s.Category == CategoryComment {
			commentCount++
		}
	}

	if commentCount < 2 {
		t.Errorf("Expected at least 2 comments, got %d", commentCount)
	}
}

func TestHighlighter_GoTypes(t *testing.T) {
	h := New()
	defer h.Close()

	code := []byte(`package test

type MyStruct struct {
	Name string
	Age  int
}

func foo(s MyStruct) MyStruct {
	return s
}
`)

	spans, err := h.Highlight("test.go", code)
	if err != nil {
		t.Fatalf("Highlight failed: %v", err)
	}

	// Check for type identifiers
	types := []string{}
	for _, s := range spans {
		if s.Category == CategoryType {
			types = append(types, string(code[s.Start:s.End]))
		}
	}

	// Should find MyStruct and built-in types
	if len(types) == 0 {
		t.Error("Expected type identifiers")
	}

	// MyStruct should appear multiple times
	myStructCount := 0
	for _, typ := range types {
		if typ == "MyStruct" {
			myStructCount++
		}
	}
	if myStructCount < 2 {
		t.Errorf("Expected MyStruct at least twice, found %d times", myStructCount)
	}
}

func TestHighlighter_UnknownLanguage(t *testing.T) {
	h := New()
	defer h.Close()

	code := []byte("some content")

	spans, err := h.Highlight("test.xyz", code)
	if err != nil {
		t.Fatalf("Highlight failed: %v", err)
	}

	if spans != nil {
		t.Errorf("Expected nil spans for unknown language, got %v", spans)
	}
}

func TestHighlighter_SupportsFile(t *testing.T) {
	h := New()
	defer h.Close()

	tests := []struct {
		filename string
		expected bool
	}{
		{"test.go", true},
		{"main.go", true},
		{"TEST.GO", true}, // case insensitive
		{"test.py", false},
		{"test.js", false},
		{"test", false},
	}

	for _, tc := range tests {
		got := h.SupportsFile(tc.filename)
		if got != tc.expected {
			t.Errorf("SupportsFile(%q) = %v, want %v", tc.filename, got, tc.expected)
		}
	}
}

func TestSpansForLine(t *testing.T) {
	// Create some test spans
	spans := []Span{
		{Start: 0, End: 7, Category: CategoryKeyword},    // "package"
		{Start: 8, End: 12, Category: CategoryNamespace}, // "main"
		{Start: 14, End: 18, Category: CategoryKeyword},  // "func"
		{Start: 19, End: 23, Category: CategoryFunction}, // "main"
	}

	// Line 0: "package main\n" (bytes 0-13)
	// Line 1: "func main()\n" (bytes 14-25)

	line0Spans := SpansForLine(spans, 0, 13)
	if len(line0Spans) != 2 {
		t.Errorf("Expected 2 spans for line 0, got %d", len(line0Spans))
	}

	// Check offsets are line-relative
	if len(line0Spans) > 0 && line0Spans[0].Start != 0 {
		t.Errorf("Expected first span to start at 0, got %d", line0Spans[0].Start)
	}

	line1Spans := SpansForLine(spans, 14, 26)
	if len(line1Spans) != 2 {
		t.Errorf("Expected 2 spans for line 1, got %d", len(line1Spans))
	}

	// Check offsets are line-relative
	if len(line1Spans) > 0 && line1Spans[0].Start != 0 {
		t.Errorf("Expected first span on line 1 to start at 0, got %d", line1Spans[0].Start)
	}
}

func TestSpansForLine_SpanningMultipleLines(t *testing.T) {
	// A span that crosses line boundaries (like a multi-line comment)
	spans := []Span{
		{Start: 5, End: 25, Category: CategoryComment},
	}

	// Line 0: bytes 0-10
	// Line 1: bytes 11-20
	// Line 2: bytes 21-30

	line0Spans := SpansForLine(spans, 0, 10)
	if len(line0Spans) != 1 {
		t.Fatalf("Expected 1 span for line 0, got %d", len(line0Spans))
	}
	// Should be clipped: starts at 5, ends at 10 (line end)
	if line0Spans[0].Start != 5 || line0Spans[0].End != 10 {
		t.Errorf("Expected span [5,10), got [%d,%d)", line0Spans[0].Start, line0Spans[0].End)
	}

	line1Spans := SpansForLine(spans, 11, 20)
	if len(line1Spans) != 1 {
		t.Fatalf("Expected 1 span for line 1, got %d", len(line1Spans))
	}
	// Should cover entire line: 0 to 9 (relative)
	if line1Spans[0].Start != 0 || line1Spans[0].End != 9 {
		t.Errorf("Expected span [0,9), got [%d,%d)", line1Spans[0].Start, line1Spans[0].End)
	}
}

func TestTheme(t *testing.T) {
	theme := DefaultTheme()

	// Check that all main categories have styles
	cats := []Category{
		CategoryKeyword,
		CategoryString,
		CategoryNumber,
		CategoryComment,
		CategoryFunction,
		CategoryType,
	}

	for _, cat := range cats {
		style := theme.Style(cat)
		// Style should exist (not panic)
		_ = style.Render("test")
	}

	// Unknown category should return empty style
	style := theme.Style(Category(999))
	rendered := style.Render("test")
	if rendered != "test" {
		t.Errorf("Expected plain 'test' for unknown category, got %q", rendered)
	}
}

func TestCategory_String(t *testing.T) {
	tests := []struct {
		cat  Category
		want string
	}{
		{CategoryNone, "None"},
		{CategoryKeyword, "Keyword"},
		{CategoryString, "String"},
		{CategoryFunction, "Function"},
		{CategoryType, "Type"},
		{Category(999), "Unknown"},
	}

	for _, tc := range tests {
		got := tc.cat.String()
		if got != tc.want {
			t.Errorf("Category(%d).String() = %q, want %q", tc.cat, got, tc.want)
		}
	}
}
