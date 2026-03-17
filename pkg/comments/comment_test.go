package comments

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
		ID:         "1705312200000",
		Text:       "This is a test comment\nwith multiple lines",
		File:       "src/foo.go",
		Line:       42,
		Anchor:     "abcd1234abcd1234abcd1234abcd1234",
		Created:    created,
		Updated:    updated,
		CommitSHA:  "abc123def456",
		Branch:     "feature-branch",
		BranchHead: "xyz789",
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
	if !strings.Contains(serialized, "#| This is a test comment") {
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
	if !strings.Contains(serialized, "# BRANCH_HEAD: xyz789") {
		t.Error("serialized missing BRANCH_HEAD")
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
	if parsed.BranchHead != original.BranchHead {
		t.Errorf("BranchHead mismatch: got %s, want %s", parsed.BranchHead, original.BranchHead)
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

func TestCommentSerializeStandalone(t *testing.T) {
	now := time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC)
	c := &Comment{
		ID:      "999",
		Text:    "TODO: investigate flaky tests",
		Branch:  "main",
		Created: now,
		Updated: now,
	}

	serialized := c.Serialize()

	// Should not contain patch headers
	if strings.Contains(serialized, "--- a/") {
		t.Error("standalone serialized should not contain patch header")
	}
	if strings.Contains(serialized, "+++ b/") {
		t.Error("standalone serialized should not contain patch header")
	}
	if strings.Contains(serialized, "@@ ") {
		t.Error("standalone serialized should not contain hunk header")
	}

	// Should contain comment text and metadata
	if !strings.Contains(serialized, "# COMMENT:") {
		t.Error("should contain COMMENT section")
	}
	if !strings.Contains(serialized, "TODO: investigate flaky tests") {
		t.Error("should contain comment text")
	}
	if !strings.Contains(serialized, "# BRANCH: main") {
		t.Error("should contain BRANCH")
	}

	// Round-trip
	parsed, err := ParseComment("999", serialized)
	if err != nil {
		t.Fatalf("ParseComment failed: %v", err)
	}
	if parsed.Text != "TODO: investigate flaky tests" {
		t.Errorf("text mismatch: got %q", parsed.Text)
	}
	if parsed.File != "" {
		t.Errorf("expected empty File, got %q", parsed.File)
	}
	if parsed.Line != 0 {
		t.Errorf("expected Line 0, got %d", parsed.Line)
	}
	if parsed.Branch != "main" {
		t.Errorf("expected Branch 'main', got %q", parsed.Branch)
	}
	if !parsed.IsStandalone() {
		t.Error("expected IsStandalone() to be true")
	}
}

func TestCommentIsStandalone(t *testing.T) {
	standalone := &Comment{Text: "note"}
	if !standalone.IsStandalone() {
		t.Error("expected standalone")
	}

	attached := &Comment{Text: "note", File: "foo.go", Line: 1}
	if attached.IsStandalone() {
		t.Error("expected not standalone")
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

func TestCommentUnresolvedSerialized(t *testing.T) {
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
	if !strings.Contains(serialized, "# RESOLVED: false") {
		t.Error("serialized should contain RESOLVED: false")
	}

	parsed, err := ParseComment("101", serialized)
	if err != nil {
		t.Fatalf("ParseComment failed: %v", err)
	}
	if parsed.Resolved {
		t.Error("expected Resolved=false")
	}
}

func TestCommentResolvedEditRoundTrip(t *testing.T) {
	// Start with unresolved, flip to resolved via edit
	c := &Comment{
		ID: "300", Text: "test", File: "test.go", Line: 1, Anchor: "abc",
		Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Updated: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Context: LineContext{Line: "code"},
	}
	serialized := c.Serialize()
	// Simulate user editing false → true
	edited := strings.Replace(serialized, "# RESOLVED: false", "# RESOLVED: true", 1)
	parsed, err := ParseComment("300", edited)
	if err != nil {
		t.Fatalf("ParseComment failed: %v", err)
	}
	if !parsed.Resolved {
		t.Error("expected Resolved=true after edit")
	}

	// Now flip back to false
	serialized2 := parsed.Serialize()
	edited2 := strings.Replace(serialized2, "# RESOLVED: true", "# RESOLVED: false", 1)
	parsed2, err := ParseComment("300", edited2)
	if err != nil {
		t.Fatalf("ParseComment failed: %v", err)
	}
	if parsed2.Resolved {
		t.Error("expected Resolved=false after edit back")
	}
}

func TestCommentResolvedInvalidValue(t *testing.T) {
	data := `--- a/test.go
+++ b/test.go
@@ -1,1 +1,1 @@
+code
# COMMENT:
# test
# FILE: test.go
# LINE: 1
# ANCHOR: abc
# CREATED: 2026-01-01T00:00:00Z
# UPDATED: 2026-01-01T00:00:00Z
# RESOLVED: yes
`
	_, err := ParseComment("400", data)
	if err == nil {
		t.Error("expected error for invalid RESOLVED value")
	}
	if !strings.Contains(err.Error(), "invalid RESOLVED value") {
		t.Errorf("expected error about RESOLVED value, got: %v", err)
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

func TestCommentAuthorRoundTrip(t *testing.T) {
	c := &Comment{
		ID:      "500",
		Text:    "agent comment",
		File:    "test.go",
		Line:    5,
		Anchor:  "abc",
		Author:  "Claude",
		Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Updated: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Context: LineContext{Line: "code"},
	}

	serialized := c.Serialize()
	if !strings.Contains(serialized, "# AUTHOR: Claude") {
		t.Error("serialized should contain AUTHOR: Claude")
	}

	parsed, err := ParseComment("500", serialized)
	if err != nil {
		t.Fatalf("ParseComment failed: %v", err)
	}
	if parsed.Author != "Claude" {
		t.Errorf("expected Author 'Claude', got %q", parsed.Author)
	}
}

func TestCommentAuthorOmittedWhenEmpty(t *testing.T) {
	c := &Comment{
		ID:      "501",
		Text:    "no author",
		File:    "test.go",
		Line:    1,
		Anchor:  "abc",
		Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Updated: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Context: LineContext{Line: "code"},
	}

	serialized := c.Serialize()
	if strings.Contains(serialized, "# AUTHOR:") {
		t.Error("serialized should not contain AUTHOR when empty")
	}

	parsed, err := ParseComment("501", serialized)
	if err != nil {
		t.Fatalf("ParseComment failed: %v", err)
	}
	if parsed.Author != "" {
		t.Errorf("expected empty Author, got %q", parsed.Author)
	}
}

func TestParseCommentAuthorBackwardCompat(t *testing.T) {
	// Existing blobs without AUTHOR field should parse with empty author
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
# RESOLVED: false
`
	c, err := ParseComment("502", data)
	if err != nil {
		t.Fatalf("ParseComment failed: %v", err)
	}
	if c.Author != "" {
		t.Errorf("expected empty Author for old blob, got %q", c.Author)
	}
}

func TestCommentAuthorInCommentTextNotConfused(t *testing.T) {
	// Make sure "AUTHOR:" appearing in comment text doesn't get parsed as metadata
	c := &Comment{
		ID:      "503",
		Text:    "the AUTHOR: field is new",
		File:    "test.go",
		Line:    1,
		Anchor:  "abc",
		Author:  "Bot",
		Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Updated: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Context: LineContext{Line: "code"},
	}

	serialized := c.Serialize()
	parsed, err := ParseComment("503", serialized)
	if err != nil {
		t.Fatalf("ParseComment failed: %v", err)
	}
	if parsed.Author != "Bot" {
		t.Errorf("expected Author 'Bot', got %q", parsed.Author)
	}
	if parsed.Text != "the AUTHOR: field is new" {
		t.Errorf("expected text preserved, got %q", parsed.Text)
	}
}

func TestParseCommentUnknownFieldsIgnored(t *testing.T) {
	t.Run("before COMMENT in metadata section", func(t *testing.T) {
		// Unknown fields before COMMENT: are silently skipped by the
		// top-level parser (they don't match any known prefix).
		data := `--- a/test.go
+++ b/test.go
@@ -1,1 +1,1 @@
+code
# ID: 600
# CREATED: 2026-01-01T00:00:00Z
# UPDATED: 2026-01-01T00:00:00Z
# PRIORITY: high
# RESOLVED: false
# CATEGORY: bug
# FILE: test.go
# LINE: 1
# ANCHOR: abc
# COMMENT:
# hello world
`
		c, err := ParseComment("600", data)
		if err != nil {
			t.Fatalf("ParseComment failed: %v", err)
		}
		assert.Equal(t, "hello world", c.Text)
		assert.Equal(t, "test.go", c.File)
		assert.Equal(t, 1, c.Line)
	})

	t.Run("after COMMENT in old-format blob", func(t *testing.T) {
		// Unknown fields between comment text and known metadata in
		// old-format blobs (# prefix) must not leak into comment text.
		data := `--- a/test.go
+++ b/test.go
@@ -1,1 +1,1 @@
+code
# ID: 601
# COMMENT:
# new matt test
# BRANCH_HEAD: 17b17f73f03b9be2c9e0832f205b7a323b46ecec
# CREATED: 2026-01-01T00:00:00Z
# UPDATED: 2026-01-01T00:00:00Z
# RESOLVED: false
# FILE: test.go
# LINE: 1
# ANCHOR: abc
`
		c, err := ParseComment("601", data)
		if err != nil {
			t.Fatalf("ParseComment failed: %v", err)
		}
		assert.Equal(t, "new matt test", c.Text)
		assert.Equal(t, "test.go", c.File)
		assert.Equal(t, 1, c.Line)
	})

	t.Run("old-format comment text with all-caps words preserved", func(t *testing.T) {
		// Comment text starting with all-caps words (e.g. "FIX:", "TODO:")
		// must not be mistakenly stripped as unknown metadata.
		data := `--- a/test.go
+++ b/test.go
@@ -1,1 +1,1 @@
+code
# ID: 602
# COMMENT:
# FIX: use the correct buffer size
# NOTE: this is important
# CREATED: 2026-01-01T00:00:00Z
# UPDATED: 2026-01-01T00:00:00Z
# RESOLVED: false
# FILE: test.go
# LINE: 1
# ANCHOR: abc
`
		c, err := ParseComment("602", data)
		if err != nil {
			t.Fatalf("ParseComment failed: %v", err)
		}
		assert.Equal(t, "FIX: use the correct buffer size\nNOTE: this is important", c.Text)
	})
}

func TestParseOldFormatNonCreatedFieldEndsComment(t *testing.T) {
	// Old-format blob where a non-CREATED metadata field is the first one
	// after COMMENT text. The field that ends the comment section must still
	// have its value correctly parsed (not silently lost).
	data := `--- a/test.go
+++ b/test.go
@@ -1,1 +1,1 @@
+code
# ID: 700
# COMMENT:
# review this
# FILE: test.go
# LINE: 42
# ANCHOR: def456
# CREATED: 2026-01-01T00:00:00Z
# UPDATED: 2026-01-01T00:00:00Z
# RESOLVED: false
`
	c, err := ParseComment("700", data)
	if err != nil {
		t.Fatalf("ParseComment failed: %v", err)
	}
	assert.Equal(t, "review this", c.Text)
	assert.Equal(t, "test.go", c.File)
	assert.Equal(t, 42, c.Line)
	assert.Equal(t, "def456", c.Anchor)
	assert.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), c.Created)
}
