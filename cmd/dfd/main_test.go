package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/user/diffyduck/pkg/content"
)

func TestParseArgs_Empty(t *testing.T) {
	result := parseArgs([]string{})
	assert.Equal(t, "diff", result.cmd)
	assert.Empty(t, result.gitArgs)
	assert.Equal(t, content.ModeDiffUnstaged, result.mode)
}

func TestParseArgs_DiffDefault(t *testing.T) {
	// No subcommand defaults to diff
	res := parseArgs([]string{"--cached"})
	assert.Equal(t, "diff", res.cmd)
	assert.Equal(t, []string{"--cached"}, res.gitArgs)
	assert.Equal(t, content.ModeDiffCached, res.mode)
}

func TestParseArgs_DiffExplicit(t *testing.T) {
	result := parseArgs([]string{"diff", "--cached"})
	assert.Equal(t, "diff", result.cmd)
	assert.Equal(t, []string{"--cached"}, result.gitArgs)
	assert.Equal(t, content.ModeDiffCached, result.mode)
}

func TestParseArgs_DiffWithRef(t *testing.T) {
	result := parseArgs([]string{"diff", "HEAD~3"})
	assert.Equal(t, "diff", result.cmd)
	assert.Equal(t, []string{"HEAD~3"}, result.gitArgs)
	assert.Equal(t, content.ModeDiffRefs, result.mode)
	assert.Equal(t, "HEAD~3", result.ref1)
}

func TestParseArgs_DiffTwoRefs(t *testing.T) {
	result := parseArgs([]string{"diff", "main", "feature"})
	assert.Equal(t, "diff", result.cmd)
	assert.Equal(t, []string{"main", "feature"}, result.gitArgs)
	assert.Equal(t, content.ModeDiffRefs, result.mode)
	assert.Equal(t, "main", result.ref1)
	assert.Equal(t, "feature", result.ref2)
}

func TestParseArgs_Show(t *testing.T) {
	result := parseArgs([]string{"show"})
	assert.Equal(t, "show", result.cmd)
	assert.Empty(t, result.gitArgs)
	assert.Equal(t, content.ModeShow, result.mode)
	assert.Equal(t, "HEAD", result.ref1) // defaults to HEAD
}

func TestParseArgs_ShowWithRef(t *testing.T) {
	result := parseArgs([]string{"show", "abc123"})
	assert.Equal(t, "show", result.cmd)
	assert.Equal(t, []string{"abc123"}, result.gitArgs)
	assert.Equal(t, content.ModeShow, result.mode)
	assert.Equal(t, "abc123", result.ref1)
}

func TestParseArgs_ShowWithMultipleArgs(t *testing.T) {
	res := parseArgs([]string{"show", "HEAD~2", "--", "file.go"})
	assert.Equal(t, "show", res.cmd)
	assert.Equal(t, []string{"HEAD~2", "--", "file.go"}, res.gitArgs)
	assert.Equal(t, content.ModeShow, res.mode)
	assert.Equal(t, "HEAD~2", res.ref1)
}

func TestParseArgs_UnknownArgPassedToDiff(t *testing.T) {
	// Unknown first arg is treated as a diff arg, not a subcommand
	result := parseArgs([]string{"HEAD~3"})
	assert.Equal(t, "diff", result.cmd)
	assert.Equal(t, []string{"HEAD~3"}, result.gitArgs)
	assert.Equal(t, content.ModeDiffRefs, result.mode)
	assert.Equal(t, "HEAD~3", result.ref1)
}

func TestExtractAllFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantArgs []string
		wantAll  bool
	}{
		{
			name:     "no flag",
			args:     []string{"diff", "HEAD"},
			wantArgs: []string{"diff", "HEAD"},
			wantAll:  false,
		},
		{
			name:     "--all flag",
			args:     []string{"diff", "--all"},
			wantArgs: []string{"diff"},
			wantAll:  true,
		},
		{
			name:     "-a flag",
			args:     []string{"diff", "-a"},
			wantArgs: []string{"diff"},
			wantAll:  true,
		},
		{
			name:     "--all with other args",
			args:     []string{"diff", "--all", "--stat"},
			wantArgs: []string{"diff", "--stat"},
			wantAll:  true,
		},
		{
			name:     "-a at start",
			args:     []string{"-a", "diff"},
			wantArgs: []string{"diff"},
			wantAll:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotArgs, gotAll := extractAllFlag(tt.args)
			assert.Equal(t, tt.wantArgs, gotArgs)
			assert.Equal(t, tt.wantAll, gotAll)
		})
	}
}
