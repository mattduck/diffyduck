package comments

import (
	"strings"
	"testing"
	"time"
)

func TestLineContextComputeAnchor(t *testing.T) {
	tests := []struct {
		name    string
		context LineContext
	}{
		{
			name: "full context",
			context: LineContext{
				Above: []string{"line1", "line2"},
				Line:  "target line",
				Below: []string{"line3", "line4"},
			},
		},
		{
			name: "partial context above",
			context: LineContext{
				Above: []string{"line1"},
				Line:  "target line",
				Below: []string{"line3", "line4"},
			},
		},
		{
			name: "no context",
			context: LineContext{
				Above: []string{},
				Line:  "target line",
				Below: []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anchor := tt.context.ComputeAnchor()

			// Anchor should be 32 hex chars (16 bytes)
			if len(anchor) != 32 {
				t.Errorf("expected anchor length 32, got %d", len(anchor))
			}

			// Same context should produce same anchor
			anchor2 := tt.context.ComputeAnchor()
			if anchor != anchor2 {
				t.Errorf("same context produced different anchors: %s vs %s", anchor, anchor2)
			}
		})
	}

	// Different contexts should produce different anchors
	ctx1 := LineContext{Line: "line A"}
	ctx2 := LineContext{Line: "line B"}
	if ctx1.ComputeAnchor() == ctx2.ComputeAnchor() {
		t.Error("different contexts produced same anchor")
	}
}

func TestNewID(t *testing.T) {
	id1 := NewID()
	time.Sleep(2 * time.Millisecond)
	id2 := NewID()

	// IDs should be numeric strings
	if id1 == "" || id2 == "" {
		t.Error("NewID returned empty string")
	}

	// IDs should be different (time-based)
	if id1 == id2 {
		t.Error("NewID returned same ID twice")
	}

	// Later ID should be larger (lexicographically works for same-length numbers)
	if id2 <= id1 {
		t.Errorf("expected id2 > id1, got id1=%s, id2=%s", id1, id2)
	}
}

func TestCommentSerializeAndParse(t *testing.T) {
	created := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	updated := time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC)

	original := &Comment{
		ID:        "1705312200000",
		Text:      "This is a test comment\nwith multiple lines",
		File:      "src/foo.go",
		Line:      42,
		Anchor:    "abcd1234abcd1234abcd1234abcd1234",
		Created:   created,
		Updated:   updated,
		CommitSHA: "abc123def456",
		Branch:    "feature-branch",
		HeadSHA:   "xyz789",
		Context: LineContext{
			Above: []string{"context above 1", "context above 2"},
			Line:  "the commented line",
			Below: []string{"context below 1", "context below 2"},
		},
	}

	// Serialize
	serialized := original.Serialize()

	// Check that serialized contains expected parts
	if !strings.Contains(serialized, "--- a/src/foo.go") {
		t.Error("serialized missing file header")
	}
	if !strings.Contains(serialized, "+++ b/src/foo.go") {
		t.Error("serialized missing file header")
	}
	if !strings.Contains(serialized, " context above 1") {
		t.Error("serialized missing context above")
	}
	if !strings.Contains(serialized, "+the commented line") {
		t.Error("serialized missing commented line")
	}
	if !strings.Contains(serialized, " context below 1") {
		t.Error("serialized missing context below")
	}
	if !strings.Contains(serialized, "# COMMENT:") {
		t.Error("serialized missing COMMENT marker")
	}
	if !strings.Contains(serialized, "# This is a test comment") {
		t.Error("serialized missing comment text")
	}
	if !strings.Contains(serialized, "# CREATED: 2026-01-15T10:30:00Z") {
		t.Error("serialized missing CREATED")
	}
	if !strings.Contains(serialized, "# COMMIT: abc123def456") {
		t.Error("serialized missing COMMIT")
	}
	if !strings.Contains(serialized, "# BRANCH: feature-branch") {
		t.Error("serialized missing BRANCH")
	}
	if !strings.Contains(serialized, "# LINE: 42") {
		t.Error("serialized missing LINE")
	}
	if !strings.Contains(serialized, "# ANCHOR: abcd1234abcd1234abcd1234abcd1234") {
		t.Error("serialized missing ANCHOR")
	}

	// Parse
	parsed, err := ParseComment(original.ID, serialized)
	if err != nil {
		t.Fatalf("ParseComment failed: %v", err)
	}

	// Verify parsed matches original
	if parsed.ID != original.ID {
		t.Errorf("ID mismatch: got %s, want %s", parsed.ID, original.ID)
	}
	if parsed.Text != original.Text {
		t.Errorf("Text mismatch: got %q, want %q", parsed.Text, original.Text)
	}
	if parsed.File != original.File {
		t.Errorf("File mismatch: got %s, want %s", parsed.File, original.File)
	}
	if parsed.Line != original.Line {
		t.Errorf("Line mismatch: got %d, want %d", parsed.Line, original.Line)
	}
	if parsed.Anchor != original.Anchor {
		t.Errorf("Anchor mismatch: got %s, want %s", parsed.Anchor, original.Anchor)
	}
	if !parsed.Created.Equal(original.Created) {
		t.Errorf("Created mismatch: got %v, want %v", parsed.Created, original.Created)
	}
	if !parsed.Updated.Equal(original.Updated) {
		t.Errorf("Updated mismatch: got %v, want %v", parsed.Updated, original.Updated)
	}
	if parsed.CommitSHA != original.CommitSHA {
		t.Errorf("CommitSHA mismatch: got %s, want %s", parsed.CommitSHA, original.CommitSHA)
	}
	if parsed.Branch != original.Branch {
		t.Errorf("Branch mismatch: got %s, want %s", parsed.Branch, original.Branch)
	}
	if parsed.HeadSHA != original.HeadSHA {
		t.Errorf("HeadSHA mismatch: got %s, want %s", parsed.HeadSHA, original.HeadSHA)
	}
	if parsed.Context.Line != original.Context.Line {
		t.Errorf("Context.Line mismatch: got %q, want %q", parsed.Context.Line, original.Context.Line)
	}
	if len(parsed.Context.Above) != len(original.Context.Above) {
		t.Errorf("Context.Above length mismatch: got %d, want %d", len(parsed.Context.Above), len(original.Context.Above))
	}
	if len(parsed.Context.Below) != len(original.Context.Below) {
		t.Errorf("Context.Below length mismatch: got %d, want %d", len(parsed.Context.Below), len(original.Context.Below))
	}
}

func TestCommentSerializeMinimal(t *testing.T) {
	// Test with minimal metadata (no optional fields)
	c := &Comment{
		ID:      "123",
		Text:    "Simple comment",
		File:    "test.go",
		Line:    10,
		Anchor:  "abc123",
		Created: time.Now(),
		Updated: time.Now(),
		Context: LineContext{
			Line: "code line",
		},
	}

	serialized := c.Serialize()

	// Should not contain optional fields
	if strings.Contains(serialized, "# COMMIT:") {
		t.Error("serialized should not contain COMMIT when empty")
	}
	if strings.Contains(serialized, "# BRANCH:") {
		t.Error("serialized should not contain BRANCH when empty")
	}
	if strings.Contains(serialized, "# HEAD:") {
		t.Error("serialized should not contain HEAD when empty")
	}
}

func TestParseCommentWithEmptyContext(t *testing.T) {
	data := `--- a/test.go
+++ b/test.go
@@ -1,1 +1,1 @@
+only line
# COMMENT:
# test
# FILE: test.go
# LINE: 1
# ANCHOR: abc
# CREATED: 2026-01-01T00:00:00Z
# UPDATED: 2026-01-01T00:00:00Z
`

	c, err := ParseComment("123", data)
	if err != nil {
		t.Fatalf("ParseComment failed: %v", err)
	}

	if c.Context.Line != "only line" {
		t.Errorf("expected Line 'only line', got %q", c.Context.Line)
	}
	if len(c.Context.Above) != 0 {
		t.Errorf("expected empty Above, got %v", c.Context.Above)
	}
	if len(c.Context.Below) != 0 {
		t.Errorf("expected empty Below, got %v", c.Context.Below)
	}
}

func TestParseCommentMultiLineText(t *testing.T) {
	data := `--- a/test.go
+++ b/test.go
@@ -1,1 +1,1 @@
+code
# COMMENT:
# Line one
# Line two
# Line three
# FILE: test.go
# LINE: 1
# ANCHOR: abc
# CREATED: 2026-01-01T00:00:00Z
# UPDATED: 2026-01-01T00:00:00Z
`

	c, err := ParseComment("123", data)
	if err != nil {
		t.Fatalf("ParseComment failed: %v", err)
	}

	expected := "Line one\nLine two\nLine three"
	if c.Text != expected {
		t.Errorf("expected Text %q, got %q", expected, c.Text)
	}
}

func TestCommentResolvedRoundTrip(t *testing.T) {
	c := &Comment{
		ID:       "100",
		Text:     "resolved comment",
		File:     "test.go",
		Line:     5,
		Anchor:   "abc",
		Resolved: true,
		Created:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Updated:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Context:  LineContext{Line: "code"},
	}

	serialized := c.Serialize()
	if !strings.Contains(serialized, "# RESOLVED: true") {
		t.Error("serialized should contain RESOLVED: true")
	}

	parsed, err := ParseComment("100", serialized)
	if err != nil {
		t.Fatalf("ParseComment failed: %v", err)
	}
	if !parsed.Resolved {
		t.Error("expected Resolved=true after round-trip")
	}
}

func TestCommentUnresolvedOmitted(t *testing.T) {
	c := &Comment{
		ID:       "101",
		Text:     "unresolved comment",
		File:     "test.go",
		Line:     5,
		Anchor:   "abc",
		Resolved: false,
		Created:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Updated:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Context:  LineContext{Line: "code"},
	}

	serialized := c.Serialize()
	if strings.Contains(serialized, "RESOLVED") {
		t.Error("serialized should not contain RESOLVED when false")
	}

	parsed, err := ParseComment("101", serialized)
	if err != nil {
		t.Fatalf("ParseComment failed: %v", err)
	}
	if parsed.Resolved {
		t.Error("expected Resolved=false when absent")
	}
}

func TestParseCommentResolvedBackwardCompat(t *testing.T) {
	// Existing blobs without RESOLVED field should parse as unresolved
	data := `--- a/test.go
+++ b/test.go
@@ -1,1 +1,1 @@
+code
# COMMENT:
# old comment
# FILE: test.go
# LINE: 1
# ANCHOR: abc
# CREATED: 2026-01-01T00:00:00Z
# UPDATED: 2026-01-01T00:00:00Z
`
	c, err := ParseComment("200", data)
	if err != nil {
		t.Fatalf("ParseComment failed: %v", err)
	}
	if c.Resolved {
		t.Error("expected Resolved=false for old blob without RESOLVED field")
	}
}
