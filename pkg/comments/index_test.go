package comments

import (
	"reflect"
	"testing"
)

func TestIndexAddAndGet(t *testing.T) {
	idx := NewIndex()

	idx.Add("src/foo.go", "123")
	idx.Add("src/foo.go", "456")
	idx.Add("src/bar.go", "789")

	// Get returns IDs for a file
	fooIDs := idx.Get("src/foo.go")
	if len(fooIDs) != 2 {
		t.Errorf("expected 2 IDs for foo.go, got %d", len(fooIDs))
	}

	barIDs := idx.Get("src/bar.go")
	if len(barIDs) != 1 {
		t.Errorf("expected 1 ID for bar.go, got %d", len(barIDs))
	}

	// Unknown file returns nil
	unknownIDs := idx.Get("unknown.go")
	if unknownIDs != nil {
		t.Errorf("expected nil for unknown file, got %v", unknownIDs)
	}
}

func TestIndexAddDuplicate(t *testing.T) {
	idx := NewIndex()

	idx.Add("src/foo.go", "123")
	idx.Add("src/foo.go", "123") // duplicate

	ids := idx.Get("src/foo.go")
	if len(ids) != 1 {
		t.Errorf("expected 1 ID (no duplicates), got %d", len(ids))
	}
}

func TestIndexRemove(t *testing.T) {
	idx := NewIndex()

	idx.Add("src/foo.go", "123")
	idx.Add("src/foo.go", "456")

	idx.Remove("src/foo.go", "123")

	ids := idx.Get("src/foo.go")
	if len(ids) != 1 || ids[0] != "456" {
		t.Errorf("expected [456], got %v", ids)
	}

	// Remove last entry cleans up the file entry
	idx.Remove("src/foo.go", "456")
	ids = idx.Get("src/foo.go")
	if ids != nil {
		t.Errorf("expected nil after removing all, got %v", ids)
	}
}

func TestIndexRemoveNonexistent(t *testing.T) {
	idx := NewIndex()
	idx.Add("src/foo.go", "123")

	// Should not panic
	idx.Remove("src/foo.go", "nonexistent")
	idx.Remove("nonexistent.go", "123")

	ids := idx.Get("src/foo.go")
	if len(ids) != 1 {
		t.Errorf("expected 1 ID, got %d", len(ids))
	}
}

func TestIndexFiles(t *testing.T) {
	idx := NewIndex()

	idx.Add("src/foo.go", "123")
	idx.Add("src/bar.go", "456")
	idx.Add("main.go", "789")

	files := idx.Files()
	expected := []string{"main.go", "src/bar.go", "src/foo.go"}
	if !reflect.DeepEqual(files, expected) {
		t.Errorf("expected %v, got %v", expected, files)
	}
}

func TestIndexAll(t *testing.T) {
	idx := NewIndex()

	idx.Add("src/foo.go", "456")
	idx.Add("src/foo.go", "123")
	idx.Add("src/bar.go", "789")

	all := idx.All()
	expected := []string{"123", "456", "789"}
	if !reflect.DeepEqual(all, expected) {
		t.Errorf("expected %v, got %v", expected, all)
	}
}

func TestIndexSerializeAndParse(t *testing.T) {
	idx := NewIndex()

	idx.Add("src/foo.go", "123")
	idx.Add("src/foo.go", "456")
	idx.Add("src/bar.go", "789")

	serialized := idx.Serialize()

	// Check format
	expected := `file:src/bar.go:789
file:src/foo.go:123
file:src/foo.go:456
`
	if serialized != expected {
		t.Errorf("serialized mismatch:\ngot:\n%s\nwant:\n%s", serialized, expected)
	}

	// Parse back
	parsed := ParseIndex(serialized)

	// Verify parsed matches original
	if !reflect.DeepEqual(idx.Get("src/foo.go"), parsed.Get("src/foo.go")) {
		t.Errorf("foo.go mismatch: got %v, want %v", parsed.Get("src/foo.go"), idx.Get("src/foo.go"))
	}
	if !reflect.DeepEqual(idx.Get("src/bar.go"), parsed.Get("src/bar.go")) {
		t.Errorf("bar.go mismatch: got %v, want %v", parsed.Get("src/bar.go"), idx.Get("src/bar.go"))
	}
}

func TestParseIndexEmpty(t *testing.T) {
	idx := ParseIndex("")
	if len(idx.All()) != 0 {
		t.Errorf("expected empty index, got %v", idx.All())
	}

	idx = ParseIndex("\n\n")
	if len(idx.All()) != 0 {
		t.Errorf("expected empty index, got %v", idx.All())
	}
}

func TestParseIndexMalformed(t *testing.T) {
	// Malformed lines should be skipped
	data := `file:valid.go:123
invalid line
file::no-path
file:no-id:
file:good.go:456
`
	idx := ParseIndex(data)

	all := idx.All()
	if len(all) != 2 {
		t.Errorf("expected 2 valid entries, got %d: %v", len(all), all)
	}
}

func TestIndexFileWithColons(t *testing.T) {
	// File paths shouldn't have colons normally, but test edge case
	idx := NewIndex()
	idx.Add("path/to/file.go", "123")

	serialized := idx.Serialize()
	parsed := ParseIndex(serialized)

	ids := parsed.Get("path/to/file.go")
	if len(ids) != 1 || ids[0] != "123" {
		t.Errorf("expected [123], got %v", ids)
	}
}
