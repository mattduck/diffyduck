package tui

import (
	"strings"
	"testing"

	"github.com/user/diffyduck/pkg/highlight"
	"github.com/user/diffyduck/pkg/sidebyside"
)

func TestSyntaxHighlighting_SpansGenerated(t *testing.T) {
	// Create a model with a Go file
	files := []sidebyside.FilePair{
		{
			OldPath: "a/test.go",
			NewPath: "b/test.go",
			Pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					Right: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
				},
			},
			FoldLevel:  sidebyside.FoldExpanded,
			OldContent: []string{"package main", "", "func hello() {", "\tfmt.Println(\"Hello\")", "}"},
			NewContent: []string{"package main", "", "func hello() {", "\tfmt.Println(\"Hello\")", "}"},
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	// Manually trigger highlighting (normally done via message)
	cmd := m.RequestHighlight(0)
	msg := cmd()

	// Should get a HighlightReadyMsg
	hlMsg, ok := msg.(HighlightReadyMsg)
	if !ok {
		t.Fatalf("Expected HighlightReadyMsg, got %T", msg)
	}

	// Should have spans for both old and new content
	if len(hlMsg.OldSpans) == 0 {
		t.Error("Expected OldSpans to have entries")
	}
	if len(hlMsg.NewSpans) == 0 {
		t.Error("Expected NewSpans to have entries")
	}

	t.Logf("Got %d old spans, %d new spans", len(hlMsg.OldSpans), len(hlMsg.NewSpans))

	// Check that we have keyword spans (for "package", "func")
	hasKeyword := false
	for _, s := range hlMsg.OldSpans {
		// CategoryKeyword = 1
		if s.Category == 1 {
			hasKeyword = true
			break
		}
	}
	if !hasKeyword {
		t.Error("Expected to find keyword category in spans")
	}
}

func TestSyntaxHighlighting_GetLineSpans(t *testing.T) {
	files := []sidebyside.FilePair{
		{
			OldPath:    "a/test.go",
			NewPath:    "b/test.go",
			FoldLevel:  sidebyside.FoldExpanded,
			OldContent: []string{"package main", "", "func hello() {"},
			NewContent: []string{"package main", "", "func hello() {"},
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	// Trigger and store highlighting
	cmd := m.RequestHighlight(0)
	msg := cmd()
	hlMsg := msg.(HighlightReadyMsg)
	m.storeHighlightSpans(hlMsg)

	// Get spans for line 1 ("package main")
	spans := m.getLineSpans(0, 1, false) // fileIndex=0, lineNum=1, isOld=false
	if len(spans) == 0 {
		t.Fatal("Expected spans for line 1")
	}

	t.Logf("Line 1 spans: %+v", spans)

	// "package" should be highlighted as keyword (bytes 0-7)
	foundPackage := false
	for _, s := range spans {
		if s.Start == 0 && s.End == 7 {
			foundPackage = true
			t.Logf("Found 'package' span: category=%d", s.Category)
		}
	}
	if !foundPackage {
		t.Error("Expected to find span for 'package' keyword at bytes 0-7")
	}

	// Get spans for line 3 ("func hello() {")
	spans3 := m.getLineSpans(0, 3, false)
	if len(spans3) == 0 {
		t.Fatal("Expected spans for line 3")
	}
	t.Logf("Line 3 spans: %+v", spans3)
}

func TestSyntaxHighlighting_RenderedOutput(t *testing.T) {
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldExpanded,
			Pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					Right: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
				},
			},
			OldContent: []string{"package main"},
			NewContent: []string{"package main"},
		},
	}

	m := New(files)
	m.width = 120
	m.height = 24
	defer m.highlighter.Close()

	// Trigger and store highlighting
	cmd := m.RequestHighlight(0)
	msg := cmd()
	hlMsg := msg.(HighlightReadyMsg)
	m.storeHighlightSpans(hlMsg)

	// Debug: check if spans are stored
	t.Logf("highlightSpans map has %d entries", len(m.highlightSpans))
	if fh, ok := m.highlightSpans[0]; ok {
		t.Logf("File 0: %d old spans, %d new spans", len(fh.OldSpans), len(fh.NewSpans))
	} else {
		t.Error("No highlight spans stored for file 0")
	}

	// Debug: check what getLineSpans returns during a simulated render
	spans := m.getLineSpans(0, 1, false)
	t.Logf("getLineSpans(0, 1, false) returned %d spans", len(spans))

	// Debug: check that HasContent returns true
	t.Logf("HasContent: %v", m.files[0].HasContent())

	// Debug: check what buildRows returns
	rows := m.buildRows()
	t.Logf("buildRows returned %d rows", len(rows))
	for i, row := range rows {
		if i > 3 {
			break
		}
		t.Logf("Row %d: isHeader=%v isSeparator=%v isBlank=%v fileIndex=%d left.Num=%d right.Num=%d",
			i, row.isHeader, row.isSeparator, row.isBlank, row.fileIndex, row.pair.Left.Num, row.pair.Right.Num)
	}

	// Test applySyntaxHighlight directly to verify it works
	// (lipgloss strips ANSI codes in test environment without TTY)
	theme := m.highlighter.Theme()

	// Get style for keyword and render test text
	keywordStyle := theme.Style(spans[0].Category)
	styledKeyword := keywordStyle.Render("package")
	t.Logf("Keyword style render result: %q", styledKeyword)

	// Even without TTY, we can verify the code path works
	// by checking that applySyntaxHighlight produces output
	result := m.applySyntaxHighlight("package main", "package main", "package main", spans, 0, 0)
	t.Logf("applySyntaxHighlight result: %q", result)

	// The result should contain "package main" (even if no ANSI codes due to no TTY)
	if !strings.Contains(result, "package") || !strings.Contains(result, "main") {
		t.Errorf("applySyntaxHighlight returned unexpected result: %q", result)
	}

	// Verify spans have correct categories (s.Category is already highlight.Category)
	for _, s := range spans {
		t.Logf("Span: start=%d end=%d category=%s", s.Start, s.End, s.Category.String())
	}

	// Verify we got the expected categories
	if len(spans) >= 1 && spans[0].Category != highlight.CategoryKeyword {
		t.Errorf("Expected first span to be Keyword, got %s", spans[0].Category.String())
	}
}

func TestSyntaxHighlighting_FullFlow(t *testing.T) {
	// Simulate the complete flow: content loads -> highlighting triggered -> render
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldExpanded,
			Pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Num: 1, Content: "func hello() {", Type: sidebyside.Context},
					Right: sidebyside.Line{Num: 1, Content: "func hello() {", Type: sidebyside.Context},
				},
			},
			// Content will be "loaded" via message
		},
	}

	m := New(files)
	m.width = 120
	m.height = 24
	defer m.highlighter.Close()

	// Simulate FileContentLoadedMsg (normally sent when content fetched)
	contentMsg := FileContentLoadedMsg{
		FileIndex:  0,
		OldContent: []string{"func hello() {", "\treturn", "}"},
		NewContent: []string{"func hello() {", "\treturn", "}"},
	}

	// Process the content loaded message
	newModel, cmd := m.Update(contentMsg)
	m = newModel.(Model)

	// The cmd should be a RequestHighlight command
	if cmd == nil {
		t.Fatal("Expected RequestHighlight command after content loaded")
	}

	// Execute the highlight command
	msg := cmd()
	hlMsg, ok := msg.(HighlightReadyMsg)
	if !ok {
		t.Fatalf("Expected HighlightReadyMsg, got %T", msg)
	}

	// Process the highlight ready message
	newModel, _ = m.Update(hlMsg)
	m = newModel.(Model)

	// Verify spans are stored
	if len(m.highlightSpans) == 0 {
		t.Fatal("Expected highlight spans to be stored")
	}

	// Verify we can get line spans
	spans := m.getLineSpans(0, 1, false)
	if len(spans) == 0 {
		t.Fatal("Expected spans for line 1")
	}

	t.Logf("Full flow test: got %d spans for line 1", len(spans))
	for _, s := range spans {
		t.Logf("  Span: start=%d end=%d category=%s", s.Start, s.End, s.Category.String())
	}

	// Verify "func" is highlighted as keyword
	foundFunc := false
	for _, s := range spans {
		if s.Start == 0 && s.End == 4 && s.Category == highlight.CategoryKeyword {
			foundFunc = true
			break
		}
	}
	if !foundFunc {
		t.Error("Expected 'func' keyword span at bytes 0-4")
	}
}

func TestSyntaxHighlighting_NormalViewFromPairs(t *testing.T) {
	// In normal view (FoldNormal), full content is not loaded, but we now
	// highlight from Pairs content (done synchronously in New for first file)
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldNormal, // Normal view
			Pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					Right: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
				},
			},
			// OldContent and NewContent are nil (not loaded)
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	// Now we should get spans from pairs-based highlighting (done sync in New)
	spans := m.getLineSpans(0, 1, false)
	if spans == nil {
		t.Error("Expected spans from pairs-based highlighting in normal view")
	}

	// Check that we get the keyword "package" highlighted
	foundPackage := false
	for _, span := range spans {
		// "package" is 7 bytes long, starts at 0
		if span.Start == 0 && span.End == 7 {
			foundPackage = true
			break
		}
	}
	if !foundPackage {
		t.Error("Expected 'package' keyword to be highlighted")
	}
}

func TestSyntaxHighlighting_FullContentRequestNoSpans(t *testing.T) {
	// RequestHighlight (for full content) should return nil when content is not loaded
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldNormal,
			Pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					Right: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
				},
			},
			// OldContent and NewContent are nil (not loaded)
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	// RequestHighlight (for full content) should produce no spans when content not loaded
	cmd := m.RequestHighlight(0)
	msg := cmd()

	if msg != nil {
		hlMsg, ok := msg.(HighlightReadyMsg)
		if ok && (len(hlMsg.OldSpans) > 0 || len(hlMsg.NewSpans) > 0) {
			t.Error("Expected no spans from full-content highlighting when content is not loaded")
		}
	}
}

func TestBuildContentFromPairs(t *testing.T) {
	pairs := []sidebyside.LinePair{
		{
			Left:  sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
			Right: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
		},
		{
			Left:  sidebyside.Line{Num: 2, Content: "", Type: sidebyside.Context},
			Right: sidebyside.Line{Num: 2, Content: "", Type: sidebyside.Context},
		},
		{
			Left:  sidebyside.Line{Num: 3, Content: "func main() {", Type: sidebyside.Context},
			Right: sidebyside.Line{Num: 3, Content: "func main() {", Type: sidebyside.Context},
		},
	}

	// Test old side (left)
	content, lineStarts, lineLens := buildContentFromPairs(pairs, true)

	// Check content is concatenated correctly
	expected := "package main\n\nfunc main() {\n"
	if string(content) != expected {
		t.Errorf("Expected content %q, got %q", expected, string(content))
	}

	// Check line starts mapping
	if lineStarts[1] != 0 {
		t.Errorf("Expected line 1 to start at 0, got %d", lineStarts[1])
	}
	if lineStarts[2] != 13 { // "package main\n" = 13 bytes
		t.Errorf("Expected line 2 to start at 13, got %d", lineStarts[2])
	}
	if lineStarts[3] != 14 { // "package main\n" + "\n" = 14 bytes
		t.Errorf("Expected line 3 to start at 14, got %d", lineStarts[3])
	}

	// Check line lengths
	if lineLens[1] != 12 { // "package main"
		t.Errorf("Expected line 1 length 12, got %d", lineLens[1])
	}
	if lineLens[2] != 0 { // empty line
		t.Errorf("Expected line 2 length 0, got %d", lineLens[2])
	}
	if lineLens[3] != 13 { // "func main() {"
		t.Errorf("Expected line 3 length 13, got %d", lineLens[3])
	}
}

func TestBuildContentFromPairs_SkipsEmptyLineNumbers(t *testing.T) {
	// Test that lines with Num=0 (empty placeholder lines) are skipped
	pairs := []sidebyside.LinePair{
		{
			Left:  sidebyside.Line{Num: 1, Content: "old line", Type: sidebyside.Removed},
			Right: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty}, // empty on right
		},
		{
			Left:  sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty}, // empty on left
			Right: sidebyside.Line{Num: 1, Content: "new line", Type: sidebyside.Added},
		},
	}

	// Old side should only have "old line"
	oldContent, oldStarts, oldLens := buildContentFromPairs(pairs, true)
	if string(oldContent) != "old line\n" {
		t.Errorf("Expected old content 'old line\\n', got %q", string(oldContent))
	}
	if _, ok := oldStarts[0]; ok {
		t.Error("Should not have entry for line 0")
	}
	if oldStarts[1] != 0 {
		t.Errorf("Expected line 1 start at 0, got %d", oldStarts[1])
	}
	if oldLens[1] != 8 {
		t.Errorf("Expected line 1 length 8, got %d", oldLens[1])
	}

	// New side should only have "new line"
	newContent, newStarts, newLens := buildContentFromPairs(pairs, false)
	if string(newContent) != "new line\n" {
		t.Errorf("Expected new content 'new line\\n', got %q", string(newContent))
	}
	if newStarts[1] != 0 {
		t.Errorf("Expected line 1 start at 0, got %d", newStarts[1])
	}
	if newLens[1] != 8 {
		t.Errorf("Expected line 1 length 8, got %d", newLens[1])
	}
}

func TestBuildContentFromPairs_NonContiguousLines(t *testing.T) {
	// Simulate a diff with gaps between hunks (non-contiguous line numbers)
	pairs := []sidebyside.LinePair{
		// First hunk: lines 5-7
		{
			Left:  sidebyside.Line{Num: 5, Content: "line five", Type: sidebyside.Context},
			Right: sidebyside.Line{Num: 5, Content: "line five", Type: sidebyside.Context},
		},
		{
			Left:  sidebyside.Line{Num: 6, Content: "line six", Type: sidebyside.Removed},
			Right: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
		},
		{
			Left:  sidebyside.Line{Num: 7, Content: "line seven", Type: sidebyside.Context},
			Right: sidebyside.Line{Num: 6, Content: "line seven", Type: sidebyside.Context},
		},
		// Gap here (lines 8-19 not in diff)
		// Second hunk: lines 20-21
		{
			Left:  sidebyside.Line{Num: 20, Content: "line twenty", Type: sidebyside.Context},
			Right: sidebyside.Line{Num: 19, Content: "line twenty", Type: sidebyside.Context},
		},
	}

	content, lineStarts, lineLens := buildContentFromPairs(pairs, true)

	// Content should be all lines concatenated (no gaps in content itself)
	expected := "line five\nline six\nline seven\nline twenty\n"
	if string(content) != expected {
		t.Errorf("Expected content %q, got %q", expected, string(content))
	}

	// But line number mapping should preserve original line numbers
	if lineStarts[5] != 0 {
		t.Errorf("Line 5 should start at 0, got %d", lineStarts[5])
	}
	if lineStarts[6] != 10 { // "line five\n" = 10 bytes
		t.Errorf("Line 6 should start at 10, got %d", lineStarts[6])
	}
	if lineStarts[20] != 30 { // after "line five\nline six\nline seven\n"
		t.Errorf("Line 20 should start at 30, got %d", lineStarts[20])
	}

	// Line lengths
	if lineLens[5] != 9 {
		t.Errorf("Line 5 length should be 9, got %d", lineLens[5])
	}
	if lineLens[20] != 11 {
		t.Errorf("Line 20 length should be 11, got %d", lineLens[20])
	}
}

func TestRequestHighlightFromPairs(t *testing.T) {
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldNormal,
			Pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Num: 1, Content: "func hello() {", Type: sidebyside.Context},
					Right: sidebyside.Line{Num: 1, Content: "func hello() {", Type: sidebyside.Context},
				},
				{
					Left:  sidebyside.Line{Num: 2, Content: `	return "hello"`, Type: sidebyside.Removed},
					Right: sidebyside.Line{Num: 2, Content: `	return "world"`, Type: sidebyside.Added},
				},
			},
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	// Clear the sync-highlighted spans to test async path
	m.pairsHighlightSpans = make(map[int]*PairsFileHighlight)

	cmd := m.RequestHighlightFromPairs(0)
	msg := cmd()

	phlMsg, ok := msg.(PairsHighlightReadyMsg)
	if !ok {
		t.Fatalf("Expected PairsHighlightReadyMsg, got %T", msg)
	}

	// Should have spans
	if len(phlMsg.OldSpans) == 0 {
		t.Error("Expected OldSpans from pairs highlighting")
	}
	if len(phlMsg.NewSpans) == 0 {
		t.Error("Expected NewSpans from pairs highlighting")
	}

	// Should have line mappings
	if len(phlMsg.OldLineStarts) == 0 {
		t.Error("Expected OldLineStarts mapping")
	}
	if len(phlMsg.NewLineStarts) == 0 {
		t.Error("Expected NewLineStarts mapping")
	}

	// Check that "func" keyword is found (bytes 0-4)
	foundFunc := false
	for _, s := range phlMsg.OldSpans {
		if s.Start == 0 && s.End == 4 && s.Category == int(highlight.CategoryKeyword) {
			foundFunc = true
			break
		}
	}
	if !foundFunc {
		t.Error("Expected 'func' keyword in OldSpans")
	}
}

func TestPairsHighlighting_StorageAndRetrieval(t *testing.T) {
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldNormal,
			Pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Num: 10, Content: "package main", Type: sidebyside.Context},
					Right: sidebyside.Line{Num: 10, Content: "package main", Type: sidebyside.Context},
				},
				{
					Left:  sidebyside.Line{Num: 11, Content: "func test() {}", Type: sidebyside.Context},
					Right: sidebyside.Line{Num: 11, Content: "func test() {}", Type: sidebyside.Context},
				},
			},
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	// First file is highlighted sync in New(), so spans should already exist
	if _, ok := m.pairsHighlightSpans[0]; !ok {
		t.Fatal("Expected pairs highlight spans for file 0 after New()")
	}

	// Get spans for line 10 (package main)
	spans10 := m.getLineSpans(0, 10, false)
	if len(spans10) == 0 {
		t.Fatal("Expected spans for line 10")
	}

	// "package" should be at bytes 0-7
	foundPackage := false
	for _, s := range spans10 {
		if s.Start == 0 && s.End == 7 && s.Category == highlight.CategoryKeyword {
			foundPackage = true
			break
		}
	}
	if !foundPackage {
		t.Error("Expected 'package' keyword span for line 10")
	}

	// Get spans for line 11 (func test() {})
	spans11 := m.getLineSpans(0, 11, false)
	if len(spans11) == 0 {
		t.Fatal("Expected spans for line 11")
	}

	// "func" should be at bytes 0-4
	foundFunc := false
	for _, s := range spans11 {
		if s.Start == 0 && s.End == 4 && s.Category == highlight.CategoryKeyword {
			foundFunc = true
			break
		}
	}
	if !foundFunc {
		t.Error("Expected 'func' keyword span for line 11")
	}
}

func TestFullContentSpansTakePriority(t *testing.T) {
	// When both full content and pairs spans exist, full content should be used
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldExpanded,
			Pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					Right: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
				},
			},
			OldContent: []string{"package main", "// comment"},
			NewContent: []string{"package main", "// comment"},
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	// New() will have created pairs-based spans for file 0
	// Now trigger full content highlighting
	cmd := m.RequestHighlight(0)
	msg := cmd()
	hlMsg := msg.(HighlightReadyMsg)
	m.storeHighlightSpans(hlMsg)

	// Both should exist now
	if _, ok := m.pairsHighlightSpans[0]; !ok {
		t.Error("Expected pairs highlight spans")
	}
	if _, ok := m.highlightSpans[0]; !ok {
		t.Error("Expected full content highlight spans")
	}

	// getLineSpans should return full content spans (which include line 2)
	// Line 2 exists in full content but not in pairs
	spans2 := m.getLineSpans(0, 2, false)
	if len(spans2) == 0 {
		t.Error("Expected spans for line 2 from full content")
	}

	// Check it's a comment
	foundComment := false
	for _, s := range spans2 {
		if s.Category == highlight.CategoryComment {
			foundComment = true
			break
		}
	}
	if !foundComment {
		t.Error("Expected comment category for line 2")
	}
}

func TestPairsHighlighting_MultipleFiles(t *testing.T) {
	// Test that first file is highlighted sync, rest are available for async
	files := []sidebyside.FilePair{
		{
			OldPath: "a/first.go",
			NewPath: "b/first.go",
			Pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Num: 1, Content: "package first", Type: sidebyside.Context},
					Right: sidebyside.Line{Num: 1, Content: "package first", Type: sidebyside.Context},
				},
			},
		},
		{
			OldPath: "a/second.go",
			NewPath: "b/second.go",
			Pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Num: 1, Content: "package second", Type: sidebyside.Context},
					Right: sidebyside.Line{Num: 1, Content: "package second", Type: sidebyside.Context},
				},
			},
		},
		{
			OldPath: "a/third.go",
			NewPath: "b/third.go",
			Pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Num: 1, Content: "package third", Type: sidebyside.Context},
					Right: sidebyside.Line{Num: 1, Content: "package third", Type: sidebyside.Context},
				},
			},
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	// First file should be highlighted sync
	if _, ok := m.pairsHighlightSpans[0]; !ok {
		t.Error("Expected first file to be highlighted synchronously")
	}

	// Other files should NOT be highlighted yet (they're async)
	if _, ok := m.pairsHighlightSpans[1]; ok {
		t.Error("Second file should not be highlighted yet")
	}
	if _, ok := m.pairsHighlightSpans[2]; ok {
		t.Error("Third file should not be highlighted yet")
	}

	// Init() should return a command to highlight the rest
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Expected Init() to return highlight command for remaining files")
	}

	// Execute the batch command - this returns multiple messages
	// For testing, we'll execute RequestHighlightFromPairs directly for each remaining file
	for i := 1; i < len(files); i++ {
		hlCmd := m.RequestHighlightFromPairs(i)
		msg := hlCmd()
		if phlMsg, ok := msg.(PairsHighlightReadyMsg); ok {
			m.storePairsHighlightSpans(phlMsg)
		}
	}

	// Now all files should have spans
	for i := 0; i < len(files); i++ {
		if _, ok := m.pairsHighlightSpans[i]; !ok {
			t.Errorf("Expected file %d to have pairs highlight spans", i)
		}
	}
}

func TestPairsHighlighting_UnsupportedFileType(t *testing.T) {
	files := []sidebyside.FilePair{
		{
			OldPath: "a/data.txt",
			NewPath: "b/data.txt",
			Pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Num: 1, Content: "some text", Type: sidebyside.Context},
					Right: sidebyside.Line{Num: 1, Content: "some text", Type: sidebyside.Context},
				},
			},
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	// .txt files are not supported, so no spans should be created
	if _, ok := m.pairsHighlightSpans[0]; ok {
		t.Error("Should not have pairs highlight spans for unsupported file type")
	}

	// getLineSpans should return nil
	spans := m.getLineSpans(0, 1, false)
	if spans != nil {
		t.Error("Expected nil spans for unsupported file type")
	}
}

func TestPairsHighlighting_EmptyPairs(t *testing.T) {
	files := []sidebyside.FilePair{
		{
			OldPath: "a/empty.go",
			NewPath: "b/empty.go",
			Pairs:   []sidebyside.LinePair{}, // No pairs
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	// Should not crash, and no spans should be created
	spans := m.getLineSpans(0, 1, false)
	if spans != nil {
		t.Error("Expected nil spans for file with no pairs")
	}
}

func TestStructureExtraction_ViewGoFile(t *testing.T) {
	// Test structure extraction on actual view.go content
	// This tests that both types and methods are extracted correctly
	viewGoContent := `package tui

import "strings"

var headerStyle = lipgloss.NewStyle()

// View implements tea.Model.
func (m Model) View() string {
	return ""
}

// displayRow represents one row in the view
type displayRow struct {
	fileIndex int
	isHeader  bool
}

// buildRows creates all displayable rows from the model data.
func (m Model) buildRows() []displayRow {
	var rows []displayRow
	return rows
}

func helperFunction() {
	// standalone function
}
`
	lines := strings.Split(viewGoContent, "\n")

	files := []sidebyside.FilePair{
		{
			OldPath:    "a/view.go",
			NewPath:    "b/view.go",
			FoldLevel:  sidebyside.FoldExpanded,
			NewContent: lines,
			OldContent: lines,
			Pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Num: 1, Content: "package tui", Type: sidebyside.Context},
					Right: sidebyside.Line{Num: 1, Content: "package tui", Type: sidebyside.Context},
				},
			},
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	cmd := m.RequestHighlight(0)
	if cmd == nil {
		t.Fatal("RequestHighlight returned nil")
	}

	msg := cmd()
	hlMsg, ok := msg.(HighlightReadyMsg)
	if !ok {
		t.Fatalf("Expected HighlightReadyMsg, got %T", msg)
	}

	if len(hlMsg.NewStructure) == 0 {
		t.Fatal("No structure entries extracted")
	}

	m.storeHighlightSpans(hlMsg)

	// Log all extracted entries
	t.Log("Extracted structure entries:")
	for _, e := range m.structureMaps[0].NewStructure.Entries {
		t.Logf("  %s %s (lines %d-%d)", e.Kind, e.Name, e.StartLine, e.EndLine)
	}

	// Verify we found the type
	entries8 := m.getStructureAtLine(0, 8)
	t.Logf("Structure at line 8 (inside View method): %+v", entries8)

	entries14 := m.getStructureAtLine(0, 14)
	t.Logf("Structure at line 14 (inside displayRow): %+v", entries14)

	entries20 := m.getStructureAtLine(0, 20)
	t.Logf("Structure at line 20 (inside buildRows): %+v", entries20)

	entries25 := m.getStructureAtLine(0, 25)
	t.Logf("Structure at line 25 (inside helperFunction): %+v", entries25)

	// Verify specific entries exist
	if len(entries8) == 0 {
		t.Error("Expected to find structure at line 8 (View method)")
	}
	if len(entries14) == 0 {
		t.Error("Expected to find structure at line 14 (displayRow type)")
	}
	if len(entries20) == 0 {
		t.Error("Expected to find structure at line 20 (buildRows method)")
	}
	if len(entries25) == 0 {
		t.Error("Expected to find structure at line 25 (helperFunction)")
	}
}

func TestStructureExtraction_LineTruncationBreaksSyntax(t *testing.T) {
	// Test that line truncation (adding [...truncated]) breaks tree-sitter parsing.
	// When a line is truncated mid-syntax, tree-sitter can't determine function boundaries.
	//
	// This simulates view.go where line 187 is a long rows = append(...) that gets
	// truncated, breaking the syntax inside buildRows function.
	contentWithTruncatedLine := `package main

type MyType struct {
	field int
}

func CompleteBeforeTruncation() {
	x := 1
}

func FunctionWithTruncatedLine() {
	// This line simulates a long line that gets truncated mid-syntax
	rows = append(rows, SomeStruct{field1: val1, field2: val2, field3[...truncated]
	y := 2
}

func FunctionAfterTruncation() {
	z := 3
}
`
	lines := strings.Split(contentWithTruncatedLine, "\n")

	files := []sidebyside.FilePair{
		{
			OldPath:    "a/test.go",
			NewPath:    "b/test.go",
			FoldLevel:  sidebyside.FoldExpanded,
			NewContent: lines,
			OldContent: lines,
			Pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					Right: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
				},
			},
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	cmd := m.RequestHighlight(0)
	msg := cmd()
	hlMsg := msg.(HighlightReadyMsg)
	m.storeHighlightSpans(hlMsg)

	// Log what was extracted
	t.Log("Extracted entries (may be incomplete due to syntax errors):")
	if m.structureMaps[0] != nil && m.structureMaps[0].NewStructure != nil {
		for _, e := range m.structureMaps[0].NewStructure.Entries {
			t.Logf("  %s %s (lines %d-%d)", e.Kind, e.Name, e.StartLine, e.EndLine)
		}
	}

	// Check what was found
	entriesType := m.getStructureAtLine(0, 4)
	entriesComplete := m.getStructureAtLine(0, 8)
	entriesTruncated := m.getStructureAtLine(0, 13)
	entriesAfter := m.getStructureAtLine(0, 19)

	t.Logf("Line 4 (MyType): %+v", entriesType)
	t.Logf("Line 8 (CompleteBeforeTruncation): %+v", entriesComplete)
	t.Logf("Line 13 (FunctionWithTruncatedLine): %+v", entriesTruncated)
	t.Logf("Line 19 (FunctionAfterTruncation): %+v", entriesAfter)

	// MyType should be found (before any truncation)
	if len(entriesType) == 0 {
		t.Error("Expected to find MyType at line 4")
	}

	// CompleteBeforeTruncation should be found
	if len(entriesComplete) == 0 {
		t.Error("Expected to find CompleteBeforeTruncation at line 8")
	}

	// These may or may not be found depending on tree-sitter error recovery
	// The point is to see how truncation affects parsing
	if len(entriesTruncated) == 0 {
		t.Log("FunctionWithTruncatedLine NOT found - truncation broke parsing")
	}
	if len(entriesAfter) == 0 {
		t.Log("FunctionAfterTruncation NOT found - truncation broke parsing of subsequent functions")
	}
}

func TestStructureExtraction_TruncatedFile(t *testing.T) {
	// Simulate a file that was truncated due to size limits.
	// Tree-sitter should still parse the available content and extract
	// structure for complete functions, even if the file ends mid-syntax.
	//
	// This simulates a file truncated at line 12 - the second function
	// is incomplete (no closing brace), but the first function should
	// still have valid structure extracted.
	truncatedContent := []string{
		"package main",
		"",
		"func CompleteFunction() {",
		"	x := 1",
		"	y := 2",
		"}",
		"",
		"func IncompleteFunction() {",
		"	a := 1",
		"	b := 2",
		"	// File truncated here - no closing brace",
	}

	files := []sidebyside.FilePair{
		{
			OldPath:             "a/large.go",
			NewPath:             "b/large.go",
			FoldLevel:           sidebyside.FoldExpanded,
			NewContent:          truncatedContent,
			OldContent:          truncatedContent,
			NewContentTruncated: true, // Marked as truncated
			Pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Num: 4, Content: "	x := 1", Type: sidebyside.Context},
					Right: sidebyside.Line{Num: 4, Content: "	x := 1", Type: sidebyside.Context},
				},
			},
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	// Trigger full content highlighting (which extracts structure)
	cmd := m.RequestHighlight(0)
	if cmd == nil {
		t.Fatal("RequestHighlight returned nil command")
	}

	msg := cmd()
	hlMsg, ok := msg.(HighlightReadyMsg)
	if !ok {
		t.Fatalf("Expected HighlightReadyMsg, got %T", msg)
	}

	// Structure should be extracted despite truncation
	if len(hlMsg.NewStructure) == 0 {
		t.Fatal("Expected structure entries to be extracted from truncated content")
	}

	// Store the structure
	m.storeHighlightSpans(hlMsg)

	// Verify structureMaps was populated
	if m.structureMaps[0] == nil || m.structureMaps[0].NewStructure == nil {
		t.Fatal("structureMaps should be populated for truncated file")
	}

	// Look up structure at line 4 (inside CompleteFunction)
	entries := m.getStructureAtLine(0, 4)
	if len(entries) == 0 {
		t.Fatal("Should find structure entries at line 4 (inside CompleteFunction)")
	}

	// Verify we found CompleteFunction
	found := false
	for _, e := range entries {
		if e.Name == "CompleteFunction" && e.Kind == "func" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected to find CompleteFunction at line 4, got: %+v", entries)
	}

	// Line 9 should be inside IncompleteFunction (even though it's truncated)
	entries9 := m.getStructureAtLine(0, 9)
	if len(entries9) == 0 {
		t.Log("Note: IncompleteFunction not found at line 9 - tree-sitter may not have parsed incomplete function")
	} else {
		for _, e := range entries9 {
			t.Logf("Found at line 9: %s %s (lines %d-%d)", e.Kind, e.Name, e.StartLine, e.EndLine)
		}
	}
}
