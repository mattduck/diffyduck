package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseArgs_Empty(t *testing.T) {
	cmd, args := parseArgs([]string{})
	assert.Equal(t, "diff", cmd)
	assert.Empty(t, args)
}

func TestParseArgs_DiffDefault(t *testing.T) {
	// No subcommand defaults to diff
	cmd, args := parseArgs([]string{"--cached"})
	assert.Equal(t, "diff", cmd)
	assert.Equal(t, []string{"--cached"}, args)
}

func TestParseArgs_DiffExplicit(t *testing.T) {
	cmd, args := parseArgs([]string{"diff", "--cached"})
	assert.Equal(t, "diff", cmd)
	assert.Equal(t, []string{"--cached"}, args)
}

func TestParseArgs_DiffWithRef(t *testing.T) {
	cmd, args := parseArgs([]string{"diff", "HEAD~3"})
	assert.Equal(t, "diff", cmd)
	assert.Equal(t, []string{"HEAD~3"}, args)
}

func TestParseArgs_DiffTwoRefs(t *testing.T) {
	cmd, args := parseArgs([]string{"diff", "main", "feature"})
	assert.Equal(t, "diff", cmd)
	assert.Equal(t, []string{"main", "feature"}, args)
}

func TestParseArgs_Show(t *testing.T) {
	cmd, args := parseArgs([]string{"show"})
	assert.Equal(t, "show", cmd)
	assert.Empty(t, args)
}

func TestParseArgs_ShowWithRef(t *testing.T) {
	cmd, args := parseArgs([]string{"show", "abc123"})
	assert.Equal(t, "show", cmd)
	assert.Equal(t, []string{"abc123"}, args)
}

func TestParseArgs_ShowWithMultipleArgs(t *testing.T) {
	cmd, args := parseArgs([]string{"show", "HEAD~2", "--", "file.go"})
	assert.Equal(t, "show", cmd)
	assert.Equal(t, []string{"HEAD~2", "--", "file.go"}, args)
}

func TestParseArgs_UnknownArgPassedToDiff(t *testing.T) {
	// Unknown first arg is treated as a diff arg, not a subcommand
	cmd, args := parseArgs([]string{"HEAD~3"})
	assert.Equal(t, "diff", cmd)
	assert.Equal(t, []string{"HEAD~3"}, args)
}
