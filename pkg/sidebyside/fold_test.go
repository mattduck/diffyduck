package sidebyside

import (
	"testing"
)

func TestFoldLevel_NextLevel(t *testing.T) {
	tests := []struct {
		name     string
		current  FoldLevel
		expected FoldLevel
	}{
		{"Normal -> Expanded", FoldNormal, FoldExpanded},
		{"Expanded -> Folded", FoldExpanded, FoldFolded},
		{"Folded -> Normal", FoldFolded, FoldNormal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.current.NextLevel()
			if got != tt.expected {
				t.Errorf("FoldLevel(%d).NextLevel() = %d, want %d", tt.current, got, tt.expected)
			}
		})
	}
}

func TestFoldLevel_String(t *testing.T) {
	tests := []struct {
		level    FoldLevel
		expected string
	}{
		{FoldNormal, "Normal"},
		{FoldExpanded, "Expanded"},
		{FoldFolded, "Folded"},
		{FoldLevel(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.level.String()
			if got != tt.expected {
				t.Errorf("FoldLevel(%d).String() = %q, want %q", tt.level, got, tt.expected)
			}
		})
	}
}

func TestFilePair_FoldLevel_Default(t *testing.T) {
	// Zero value should be FoldNormal
	fp := FilePair{}
	if fp.FoldLevel != FoldNormal {
		t.Errorf("FilePair zero value FoldLevel = %v, want FoldNormal", fp.FoldLevel)
	}
}

func TestFilePair_ContentFields(t *testing.T) {
	fp := FilePair{
		OldPath:    "a/foo.go",
		NewPath:    "b/foo.go",
		FoldLevel:  FoldExpanded,
		OldContent: []string{"line1", "line2"},
		NewContent: []string{"line1", "line2", "line3"},
	}

	if fp.FoldLevel != FoldExpanded {
		t.Errorf("FoldLevel = %v, want FoldExpanded", fp.FoldLevel)
	}
	if len(fp.OldContent) != 2 {
		t.Errorf("OldContent len = %d, want 2", len(fp.OldContent))
	}
	if len(fp.NewContent) != 3 {
		t.Errorf("NewContent len = %d, want 3", len(fp.NewContent))
	}
}

func TestFilePair_HasContent(t *testing.T) {
	tests := []struct {
		name       string
		fp         FilePair
		hasContent bool
	}{
		{
			name:       "no content",
			fp:         FilePair{},
			hasContent: false,
		},
		{
			name:       "has old content only",
			fp:         FilePair{OldContent: []string{"line"}},
			hasContent: true,
		},
		{
			name:       "has new content only",
			fp:         FilePair{NewContent: []string{"line"}},
			hasContent: true,
		},
		{
			name:       "has both",
			fp:         FilePair{OldContent: []string{"a"}, NewContent: []string{"b"}},
			hasContent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fp.HasContent()
			if got != tt.hasContent {
				t.Errorf("HasContent() = %v, want %v", got, tt.hasContent)
			}
		})
	}
}
