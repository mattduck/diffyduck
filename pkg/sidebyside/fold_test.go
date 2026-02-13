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
		{"Header -> Structure", FoldHeader, FoldStructure},
		{"Structure -> Hunks", FoldStructure, FoldHunks},
		{"Hunks -> Header", FoldHunks, FoldHeader},
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
		{FoldHeader, "Header"},
		{FoldStructure, "Structure"},
		{FoldHunks, "Hunks"},
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

func TestCommitFoldLevel_NextLevel(t *testing.T) {
	tests := []struct {
		name     string
		current  CommitFoldLevel
		expected CommitFoldLevel
	}{
		{"Folded -> FileHeaders", CommitFolded, CommitFileHeaders},
		{"FileHeaders -> FileStructure", CommitFileHeaders, CommitFileStructure},
		{"FileStructure -> FileHunks", CommitFileStructure, CommitFileHunks},
		{"FileHunks -> Folded", CommitFileHunks, CommitFolded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.current.NextLevel()
			if got != tt.expected {
				t.Errorf("CommitFoldLevel(%d).NextLevel() = %d, want %d", tt.current, got, tt.expected)
			}
		})
	}
}

func TestCommitFileFold(t *testing.T) {
	tests := []struct {
		commit   CommitFoldLevel
		expected FoldLevel
	}{
		{CommitFolded, FoldHeader},
		{CommitFileHeaders, FoldHeader},
		{CommitFileStructure, FoldStructure},
		{CommitFileHunks, FoldHunks},
	}

	for _, tt := range tests {
		t.Run(tt.commit.String(), func(t *testing.T) {
			got := CommitFileFold[tt.commit]
			if got != tt.expected {
				t.Errorf("CommitFileFold[%v] = %v, want %v", tt.commit, got, tt.expected)
			}
		})
	}
}

func TestFilePair_FoldLevel_Default(t *testing.T) {
	// Zero value should be FoldHeader
	fp := FilePair{}
	if fp.FoldLevel != FoldHeader {
		t.Errorf("FilePair zero value FoldLevel = %v, want FoldHeader", fp.FoldLevel)
	}
}

func TestFilePair_ContentFields(t *testing.T) {
	fp := FilePair{
		OldPath:    "a/foo.go",
		NewPath:    "b/foo.go",
		FoldLevel:  FoldHunks,
		OldContent: []string{"line1", "line2"},
		NewContent: []string{"line1", "line2", "line3"},
	}

	if fp.FoldLevel != FoldHunks {
		t.Errorf("FoldLevel = %v, want FoldHunks", fp.FoldLevel)
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
