package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/content"
)

func TestParseArgs_Empty(t *testing.T) {
	result, err := parseArgs([]string{})
	require.NoError(t, err)
	assert.Equal(t, "diff", result.cmd)
	assert.Empty(t, result.refs)
	assert.Empty(t, result.paths)
	assert.Equal(t, content.ModeDiffUnstaged, result.mode)
}

func TestParseArgs_SubcommandDetection(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantCmd string
	}{
		{"diff explicit", []string{"diff"}, "diff"},
		{"diff default", []string{"HEAD"}, "diff"},
		{"show", []string{"show"}, "show"},
		{"log", []string{"log"}, "log"},
		{"clean", []string{"clean"}, "clean"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseArgs(tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.wantCmd, result.cmd)
		})
	}
}

func TestParseArgs_DiffWithCached(t *testing.T) {
	result, err := parseArgs([]string{"--cached"})
	require.NoError(t, err)
	assert.Equal(t, "diff", result.cmd)
	assert.True(t, result.cached)
	assert.Equal(t, content.ModeDiffCached, result.mode)
}

func TestParseArgs_DiffWithStaged(t *testing.T) {
	result, err := parseArgs([]string{"diff", "--staged"})
	require.NoError(t, err)
	assert.True(t, result.cached)
	assert.Equal(t, content.ModeDiffCached, result.mode)
}

func TestParseArgs_DiffWithRef(t *testing.T) {
	result, err := parseArgs([]string{"diff", "HEAD~3"})
	require.NoError(t, err)
	assert.Equal(t, "diff", result.cmd)
	assert.Equal(t, content.ModeDiffRefs, result.mode)
	assert.Equal(t, "HEAD~3", result.ref1)
	assert.Equal(t, "", result.ref2)
}

func TestParseArgs_DiffTwoRefs(t *testing.T) {
	result, err := parseArgs([]string{"diff", "main", "feature"})
	require.NoError(t, err)
	assert.Equal(t, content.ModeDiffRefs, result.mode)
	assert.Equal(t, "main", result.ref1)
	assert.Equal(t, "feature", result.ref2)
}

func TestParseArgs_Show(t *testing.T) {
	result, err := parseArgs([]string{"show"})
	require.NoError(t, err)
	assert.Equal(t, "show", result.cmd)
	assert.Equal(t, content.ModeShow, result.mode)
	assert.Equal(t, "HEAD", result.ref1)
}

func TestParseArgs_ShowWithRef(t *testing.T) {
	result, err := parseArgs([]string{"show", "abc123"})
	require.NoError(t, err)
	assert.Equal(t, "show", result.cmd)
	assert.Equal(t, content.ModeShow, result.mode)
	assert.Equal(t, "abc123", result.ref1)
}

func TestParseArgs_ShowWithPaths(t *testing.T) {
	result, err := parseArgs([]string{"show", "HEAD~2", "--", "file.go"})
	require.NoError(t, err)
	assert.Equal(t, "show", result.cmd)
	assert.Equal(t, "HEAD~2", result.ref1)
	assert.Equal(t, []string{"file.go"}, result.paths)
}

func TestParseArgs_UnknownArgDefaultsToDiff(t *testing.T) {
	result, err := parseArgs([]string{"HEAD~3"})
	require.NoError(t, err)
	assert.Equal(t, "diff", result.cmd)
	assert.Equal(t, content.ModeDiffRefs, result.mode)
	assert.Equal(t, "HEAD~3", result.ref1)
}

func TestParseArgs_Clean(t *testing.T) {
	result, err := parseArgs([]string{"clean"})
	require.NoError(t, err)
	assert.Equal(t, "clean", result.cmd)
}

// Flag tests

func TestParseArgs_AllFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"--all", []string{"diff", "--all"}},
		{"-a", []string{"diff", "-a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseArgs(tt.args)
			require.NoError(t, err)
			assert.True(t, result.allMode)
		})
	}
}

func TestParseArgs_UnstagedFlag(t *testing.T) {
	result, err := parseArgs([]string{"diff", "--unstaged"})
	require.NoError(t, err)
	assert.True(t, result.unstaged)
}

func TestParseArgs_SnapshotsFlags(t *testing.T) {
	t.Run("--snapshots", func(t *testing.T) {
		result, err := parseArgs([]string{"diff", "--snapshots"})
		require.NoError(t, err)
		require.NotNil(t, result.snapshots)
		assert.True(t, *result.snapshots)
	})
	t.Run("--no-snapshots", func(t *testing.T) {
		result, err := parseArgs([]string{"diff", "--no-snapshots"})
		require.NoError(t, err)
		require.NotNil(t, result.snapshots)
		assert.False(t, *result.snapshots)
	})
	t.Run("default nil", func(t *testing.T) {
		result, err := parseArgs([]string{"diff"})
		require.NoError(t, err)
		assert.Nil(t, result.snapshots)
	})
}

func TestParseArgs_DebugFlag(t *testing.T) {
	result, err := parseArgs([]string{"diff", "--debug"})
	require.NoError(t, err)
	assert.True(t, result.debug)
}

func TestParseArgs_CPUProfileFlag(t *testing.T) {
	t.Run("equals form", func(t *testing.T) {
		result, err := parseArgs([]string{"diff", "--cpuprofile=/tmp/prof"})
		require.NoError(t, err)
		assert.Equal(t, "/tmp/prof", result.cpuProfile)
	})
	t.Run("space form", func(t *testing.T) {
		result, err := parseArgs([]string{"diff", "--cpuprofile", "/tmp/prof"})
		require.NoError(t, err)
		assert.Equal(t, "/tmp/prof", result.cpuProfile)
	})
}

// Exclude flag tests

func TestParseArgs_ExcludeFlags(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantExcludes []string
	}{
		{
			"--exclude=glob",
			[]string{"diff", "--exclude=*.md"},
			[]string{"*.md"},
		},
		{
			"--exclude glob",
			[]string{"diff", "--exclude", "*.md"},
			[]string{"*.md"},
		},
		{
			"-e glob",
			[]string{"diff", "-e", "*.md"},
			[]string{"*.md"},
		},
		{
			"-eglob (attached)",
			[]string{"diff", "-e*.md"},
			[]string{"*.md"},
		},
		{
			"multiple excludes",
			[]string{"diff", "-e", "*.md", "--exclude=vendor/**"},
			[]string{"*.md", "vendor/**"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseArgs(tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.wantExcludes, result.excludes)
		})
	}
}

// Count flag tests (-n for log)

func TestParseArgs_CountFlag(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantCount int
	}{
		{"-n 20", []string{"log", "-n", "20"}, 20},
		{"-n20", []string{"log", "-n20"}, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseArgs(tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.wantCount, result.count)
		})
	}
}

// Separator and paths tests

func TestParseArgs_PathsAfterSeparator(t *testing.T) {
	result, err := parseArgs([]string{"diff", "main", "--", "src/", "*.go"})
	require.NoError(t, err)
	assert.Equal(t, []string{"main"}, result.refs)
	assert.Equal(t, []string{"src/", "*.go"}, result.paths)
}

func TestParseArgs_PathsAndExcludes(t *testing.T) {
	result, err := parseArgs([]string{"diff", "-e", "vendor/**", "--", "src/"})
	require.NoError(t, err)
	assert.Equal(t, []string{"src/"}, result.paths)
	assert.Equal(t, []string{"vendor/**"}, result.excludes)
}

// Log with ref range

func TestParseArgs_LogRefRange(t *testing.T) {
	result, err := parseArgs([]string{"log", "main..feature"})
	require.NoError(t, err)
	assert.Equal(t, "log", result.cmd)
	assert.Equal(t, "main..feature", result.ref1)
}

func TestParseArgs_LogWithCountAndRange(t *testing.T) {
	result, err := parseArgs([]string{"log", "-n", "10", "main..feature"})
	require.NoError(t, err)
	assert.Equal(t, 10, result.count)
	assert.Equal(t, "main..feature", result.ref1)
}

// Error cases

func TestParseArgs_UnknownFlag(t *testing.T) {
	_, err := parseArgs([]string{"diff", "--stat"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown flag: --stat")
}

func TestParseArgs_TooManyRefs(t *testing.T) {
	_, err := parseArgs([]string{"diff", "a", "b", "c"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at most 2 refs")
}

func TestParseArgs_ConflictingFlags(t *testing.T) {
	_, err := parseArgs([]string{"diff", "--cached", "--unstaged"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be used together")
}

func TestParseArgs_CachedWithRefs(t *testing.T) {
	_, err := parseArgs([]string{"diff", "--cached", "main"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--cached cannot be used with ref arguments")
}

func TestParseArgs_ShowTooManyRefs(t *testing.T) {
	_, err := parseArgs([]string{"show", "a", "b"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at most 1 ref")
}

func TestParseArgs_CountOnDiff(t *testing.T) {
	_, err := parseArgs([]string{"diff", "-n", "5"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "-n is only valid for log")
}

func TestParseArgs_InvalidCountValue(t *testing.T) {
	_, err := parseArgs([]string{"log", "-n", "abc"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "positive integer")
}

func TestParseArgs_CachedOnShow(t *testing.T) {
	_, err := parseArgs([]string{"show", "--cached"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only valid for diff")
}

func TestParseArgs_SnapshotsOnLog(t *testing.T) {
	_, err := parseArgs([]string{"log", "--snapshots"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only valid for diff")
}

func TestParseArgs_CleanWithArgs(t *testing.T) {
	_, err := parseArgs([]string{"clean", "foo"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not accept arguments")
}

func TestParseArgs_MissingExcludeValue(t *testing.T) {
	_, err := parseArgs([]string{"diff", "-e"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires a glob pattern")
}

func TestParseArgs_MissingCountValue(t *testing.T) {
	_, err := parseArgs([]string{"log", "-n"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires a count")
}

// buildPathspec tests

func TestBuildPathspec(t *testing.T) {
	tests := []struct {
		name     string
		paths    []string
		excludes []string
		want     []string
	}{
		{
			"empty",
			nil, nil, nil,
		},
		{
			"paths only",
			[]string{"src/", "*.go"}, nil,
			[]string{"--", "src/", "*.go"},
		},
		{
			"excludes only",
			nil, []string{"vendor/**"},
			[]string{"--", ":!vendor/**"},
		},
		{
			"combined",
			[]string{"src/"}, []string{"vendor/**", "*.test.ts"},
			[]string{"--", "src/", ":!vendor/**", ":!*.test.ts"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPathspec(tt.paths, tt.excludes)
			assert.Equal(t, tt.want, got)
		})
	}
}

// filterFiles tests

func TestFilterFiles(t *testing.T) {
	files := []string{
		"src/main.go",
		"src/util.go",
		"vendor/lib/foo.go",
		"README.md",
		"docs/guide.md",
	}

	tests := []struct {
		name     string
		paths    []string
		excludes []string
		want     []string
	}{
		{
			"no filters",
			nil, nil,
			files,
		},
		{
			"path prefix filter",
			[]string{"src/"}, nil,
			[]string{"src/main.go", "src/util.go"},
		},
		{
			"glob filter",
			[]string{"*.md"}, nil,
			[]string{"README.md", "docs/guide.md"},
		},
		{
			"exclude glob",
			nil, []string{"*.md"},
			[]string{"src/main.go", "src/util.go", "vendor/lib/foo.go"},
		},
		{
			"exclude ** pattern",
			nil, []string{"vendor/**"},
			[]string{"src/main.go", "src/util.go", "README.md", "docs/guide.md"},
		},
		{
			"path + exclude combined",
			[]string{"src/"}, []string{"*_test.go"},
			[]string{"src/main.go", "src/util.go"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterFiles(files, tt.paths, tt.excludes)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Combined flags tests

func TestParseArgs_CombinedFlags(t *testing.T) {
	result, err := parseArgs([]string{
		"diff", "--all", "--debug", "--no-snapshots",
		"-e", "vendor/**", "--exclude=*.md",
		"--", "src/",
	})
	require.NoError(t, err)
	assert.Equal(t, "diff", result.cmd)
	assert.True(t, result.allMode)
	assert.True(t, result.debug)
	assert.NotNil(t, result.snapshots)
	assert.False(t, *result.snapshots)
	assert.Equal(t, []string{"vendor/**", "*.md"}, result.excludes)
	assert.Equal(t, []string{"src/"}, result.paths)
}
