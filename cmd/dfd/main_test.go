package main

import (
	"bytes"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/internal/tui"
	"github.com/user/diffyduck/pkg/comments"
	"github.com/user/diffyduck/pkg/config"
	"github.com/user/diffyduck/pkg/content"
	"github.com/user/diffyduck/pkg/highlight"
)

// stripANSI removes ANSI escape codes for test assertions.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func TestParseArgs_Empty(t *testing.T) {
	res, err := parseArgs([]string{})
	require.NoError(t, err)
	assert.Equal(t, "status", res.cmd)
}

func TestParseArgs_SubcommandDetection(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantCmd string
	}{
		{"diff explicit", []string{"diff"}, "diff"},
		{"diff alias", []string{"d"}, "diff"},
		{"diff default", []string{"diff", "HEAD"}, "diff"},
		{"show", []string{"show"}, "show"},
		{"log", []string{"log"}, "log"},
		{"log alias", []string{"l"}, "log"},
		{"clean", []string{"clean"}, "clean"},
		{"branch", []string{"branch"}, "branch"},
		{"branch alias", []string{"b"}, "branch"},
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
	result, err := parseArgs([]string{"diff", "--cached"})
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
	result, err := parseArgs([]string{"diff", "HEAD~3"})
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

func TestParseArgs_BranchWithArgs(t *testing.T) {
	_, err := parseArgs([]string{"branch", "main"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not accept arguments")
}

func TestParseArgs_BranchWithFlags(t *testing.T) {
	_, err := parseArgs([]string{"branch", "--cached"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only accepts -v/--verbose and --since")
}

func TestParseArgs_BranchVerbose(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"-v", []string{"branch", "-v"}},
		{"--verbose", []string{"branch", "--verbose"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseArgs(tt.args)
			require.NoError(t, err)
			assert.Equal(t, "branch", result.cmd)
			assert.True(t, result.verbose)
		})
	}
}

func TestParseArgs_BranchSince(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		since string
	}{
		{"--since=7d", []string{"branch", "--since=7d"}, "7d"},
		{"--since 2w", []string{"branch", "--since", "2w"}, "2w"},
		{"--since=all", []string{"branch", "--since=all"}, "all"},
		{"--since=1y", []string{"branch", "--since=1y"}, "1y"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseArgs(tt.args)
			require.NoError(t, err)
			assert.Equal(t, "branch", result.cmd)
			assert.Equal(t, tt.since, result.since)
		})
	}
}

func TestParseArgs_LogSince(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		since string
	}{
		{"--since=7d", []string{"log", "--since=7d"}, "7d"},
		{"--since 2w", []string{"log", "--since", "2w"}, "2w"},
		{"--since=all", []string{"log", "--since=all"}, "all"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseArgs(tt.args)
			require.NoError(t, err)
			assert.Equal(t, "log", result.cmd)
			assert.Equal(t, tt.since, result.since)
		})
	}
}

func TestParseArgs_SinceOnDiff(t *testing.T) {
	_, err := parseArgs([]string{"diff", "--since=7d"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only valid for branch, log, and comment")
}

func TestParseArgs_SinceMissingValue(t *testing.T) {
	_, err := parseArgs([]string{"branch", "--since"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires a duration")
}

func TestParseSinceDuration(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
		err   bool
	}{
		{"30m", 30 * time.Minute, false},
		{"6h", 6 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"3M", 90 * 24 * time.Hour, false},
		{"1y", 365 * 24 * time.Hour, false},
		{"all", 0, false},
		{"", 0, false},
		{"bad", 0, true},
		{"0d", 0, true},
		{"-1d", 0, true},
		{"3x", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseSinceDuration(tt.input)
			if tt.err {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestParseArgs_VerboseOnDiff(t *testing.T) {
	_, err := parseArgs([]string{"diff", "-v"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only valid for branch and comment")
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

// Help and version tests

func TestParseArgs_HelpFlag(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		helpCmd string
	}{
		{"--help bare", []string{"--help"}, ""},
		{"-h bare", []string{"-h"}, ""},
		{"diff --help", []string{"diff", "--help"}, "diff"},
		{"diff -h", []string{"diff", "-h"}, "diff"},
		{"d --help", []string{"d", "--help"}, "diff"},
		{"show --help", []string{"show", "--help"}, "show"},
		{"log -h", []string{"log", "-h"}, "log"},
		{"l -h", []string{"l", "-h"}, "log"},
		{"b --help", []string{"b", "--help"}, "branch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseArgs(tt.args)
			require.NoError(t, err)
			assert.True(t, result.showHelp)
			assert.Equal(t, tt.helpCmd, result.helpCmd)
		})
	}
}

func TestParseArgs_HelpSubcommand(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		helpCmd string
	}{
		{"help bare", []string{"help"}, ""},
		{"help diff", []string{"help", "diff"}, "diff"},
		{"help d", []string{"help", "d"}, "diff"},
		{"help show", []string{"help", "show"}, "show"},
		{"help log", []string{"help", "log"}, "log"},
		{"help l", []string{"help", "l"}, "log"},
		{"help clean", []string{"help", "clean"}, "clean"},
		{"help branch", []string{"help", "branch"}, "branch"},
		{"help b", []string{"help", "b"}, "branch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseArgs(tt.args)
			require.NoError(t, err)
			assert.True(t, result.showHelp)
			assert.Equal(t, tt.helpCmd, result.helpCmd)
		})
	}
}

func TestParseArgs_VersionFlag(t *testing.T) {
	result, err := parseArgs([]string{"--version"})
	require.NoError(t, err)
	assert.True(t, result.showVersion)
}

func TestParseArgs_HelpSkipsValidation(t *testing.T) {
	// --help should not fail even with otherwise invalid flag combos
	result, err := parseArgs([]string{"diff", "--cached", "--unstaged", "--help"})
	require.NoError(t, err)
	assert.True(t, result.showHelp)
}

func TestPrintUsage_General(t *testing.T) {
	output := captureStdout(func() {
		printUsage("")
	})
	assert.Contains(t, output, "dfd - terminal side-by-side diff viewer")
	assert.Contains(t, output, "Commands:")
	assert.Contains(t, output, "diff, d")
	assert.Contains(t, output, "show")
	assert.Contains(t, output, "log, l")
	assert.Contains(t, output, "branch, b")
	assert.Contains(t, output, "--help")
	assert.Contains(t, output, "--version")
}

func TestPrintUsage_Subcommands(t *testing.T) {
	tests := []struct {
		cmd      string
		contains []string
	}{
		{"diff", []string{"dfd diff", "--cached", "--exclude", "Examples:"}},
		{"show", []string{"dfd show", "Defaults to HEAD", "Examples:"}},
		{"log", []string{"dfd log", "-n <count>", "Examples:"}},
		{"clean", []string{"dfd clean", "snapshot"}},
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			output := captureStdout(func() {
				printUsage(tt.cmd)
			})
			for _, s := range tt.contains {
				assert.Contains(t, output, s)
			}
		})
	}
}

func TestPrintUsage_UnknownCmdFallsToGeneral(t *testing.T) {
	output := captureStdout(func() {
		printUsage("nonexistent")
	})
	assert.Contains(t, output, "dfd - terminal side-by-side diff viewer")
}

// captureStdout captures stdout output from a function call.
func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
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

func TestParseArgs_Config(t *testing.T) {
	result, err := parseArgs([]string{"config"})
	require.NoError(t, err)
	assert.Equal(t, "config", result.cmd)
	assert.False(t, result.configInit)
	assert.False(t, result.configPrint)
	assert.False(t, result.configPath)
}

func TestParseArgs_ConfigInit(t *testing.T) {
	result, err := parseArgs([]string{"config", "--init"})
	require.NoError(t, err)
	assert.True(t, result.configInit)
	assert.False(t, result.configForce)
}

func TestParseArgs_ConfigInitForce(t *testing.T) {
	result, err := parseArgs([]string{"config", "--init", "--force"})
	require.NoError(t, err)
	assert.True(t, result.configInit)
	assert.True(t, result.configForce)
}

func TestParseArgs_ConfigPrint(t *testing.T) {
	result, err := parseArgs([]string{"config", "--print"})
	require.NoError(t, err)
	assert.True(t, result.configPrint)
}

func TestParseArgs_ConfigPath(t *testing.T) {
	result, err := parseArgs([]string{"config", "--path"})
	require.NoError(t, err)
	assert.True(t, result.configPath)
}

func TestParseArgs_ConfigForceWithoutInit(t *testing.T) {
	_, err := parseArgs([]string{"config", "--force"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--force can only be used with --init")
}

func TestParseArgs_ConfigInitAndPath(t *testing.T) {
	_, err := parseArgs([]string{"config", "--init", "--path"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--init and --path cannot be used together")
}

func TestParseArgs_ConfigPrintAndPath(t *testing.T) {
	_, err := parseArgs([]string{"config", "--print", "--path"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--print and --path cannot be used together")
}

func TestParseArgs_ConfigEdit(t *testing.T) {
	result, err := parseArgs([]string{"config", "--edit"})
	require.NoError(t, err)
	assert.True(t, result.configEdit)
}

func TestParseArgs_ConfigEditConflicts(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"edit+init", []string{"config", "--edit", "--init"}},
		{"edit+print", []string{"config", "--edit", "--print"}},
		{"edit+path", []string{"config", "--edit", "--path"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseArgs(tt.args)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "--edit cannot be combined")
		})
	}
}

func TestParseArgs_ConfigFlagsOnDiff(t *testing.T) {
	_, err := parseArgs([]string{"diff", "--init"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only valid for config command")
}

func TestParseArgs_ConfigEditOnDiff(t *testing.T) {
	_, err := parseArgs([]string{"diff", "--edit"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only valid for config command")
}

func TestParseArgs_ConfigWithArgs(t *testing.T) {
	_, err := parseArgs([]string{"config", "foo"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not accept arguments")
}

func TestEditorCmd(t *testing.T) {
	// Save and restore env
	origVisual := os.Getenv("VISUAL")
	origEditor := os.Getenv("EDITOR")
	t.Cleanup(func() {
		os.Setenv("VISUAL", origVisual)
		os.Setenv("EDITOR", origEditor)
	})

	t.Run("VISUAL takes precedence", func(t *testing.T) {
		os.Setenv("VISUAL", "code")
		os.Setenv("EDITOR", "vim")
		assert.Equal(t, "code", editorCmd())
	})

	t.Run("EDITOR fallback", func(t *testing.T) {
		os.Unsetenv("VISUAL")
		os.Setenv("EDITOR", "vim")
		assert.Equal(t, "vim", editorCmd())
	})

	t.Run("empty when neither set", func(t *testing.T) {
		os.Unsetenv("VISUAL")
		os.Unsetenv("EDITOR")
		assert.Equal(t, "", editorCmd())
	})
}

// --- Comment subcommand tests ---

func TestParseArgs_Comment(t *testing.T) {
	result, err := parseArgs([]string{"comment"})
	require.NoError(t, err)
	assert.Equal(t, "comment", result.cmd)
	assert.Equal(t, "list", result.commentSub)
}

func TestParseArgs_CommentAlias(t *testing.T) {
	result, err := parseArgs([]string{"c"})
	require.NoError(t, err)
	assert.Equal(t, "comment", result.cmd)
}

func TestParseArgs_CommentList(t *testing.T) {
	result, err := parseArgs([]string{"comment", "list"})
	require.NoError(t, err)
	assert.Equal(t, "comment", result.cmd)
	assert.Equal(t, "list", result.commentSub)
	assert.False(t, result.commentNSet)
}

func TestParseArgs_CommentListN(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantN   int
		wantSet bool
	}{
		{"positive", []string{"comment", "list", "-n", "10"}, 10, true},
		{"negative", []string{"comment", "list", "-n", "-3"}, -3, true},
		{"zero", []string{"comment", "list", "-n", "0"}, 0, true},
		{"bare -n", []string{"comment", "list", "-n"}, 0, true},
		{"bare -n before flag", []string{"comment", "list", "-n", "--verbose"}, 0, true}, // also check commentVerbose below
		{"attached positive", []string{"comment", "list", "-n10"}, 10, true},
		{"attached negative", []string{"c", "list", "-n-5"}, -5, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseArgs(tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.wantN, result.commentN)
			assert.Equal(t, tt.wantSet, result.commentNSet)
		})
	}

	// Verify bare -n doesn't swallow the following flag.
	t.Run("bare -n before flag parses verbose", func(t *testing.T) {
		result, err := parseArgs([]string{"comment", "list", "-n", "--verbose"})
		require.NoError(t, err)
		assert.True(t, result.commentVerbose)
	})
}

func TestParseArgs_CommentListStatus(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		wantSt string
	}{
		{"unresolved", []string{"comment", "list", "--status", "unresolved"}, "unresolved"},
		{"resolved", []string{"comment", "list", "--status", "resolved"}, "resolved"},
		{"all", []string{"comment", "list", "--status", "all"}, "all"},
		{"equals form", []string{"comment", "list", "--status=resolved"}, "resolved"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseArgs(tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.wantSt, result.commentStatus)
		})
	}
}

func TestParseArgs_CommentListStatusInvalid(t *testing.T) {
	_, err := parseArgs([]string{"comment", "list", "--status", "bogus"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be unresolved, resolved, or all")
}

func TestParseArgs_CommentEdit(t *testing.T) {
	result, err := parseArgs([]string{"comment", "edit", "1705312200000"})
	require.NoError(t, err)
	assert.Equal(t, "edit", result.commentSub)
	assert.Equal(t, "1705312200000", result.commentID)
}

func TestParseArgs_CommentListSuffix(t *testing.T) {
	result, err := parseArgs([]string{"comment", "list", "7415"})
	require.NoError(t, err)
	assert.Equal(t, "list", result.commentSub)
	assert.Equal(t, "7415", result.commentID)
}

func TestParseArgs_CommentEditResolved(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"equals true", []string{"comment", "edit", "abc", "--resolved=true"}, true},
		{"equals false", []string{"comment", "edit", "abc", "--resolved=false"}, false},
		{"space true", []string{"comment", "edit", "abc", "--resolved", "true"}, true},
		{"space false", []string{"comment", "edit", "abc", "--resolved", "false"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseArgs(tt.args)
			require.NoError(t, err)
			assert.Equal(t, "edit", result.commentSub)
			assert.Equal(t, "abc", result.commentID)
			require.NotNil(t, result.commentResolved)
			assert.Equal(t, tt.want, *result.commentResolved)
		})
	}
}

func TestParseArgs_CommentEditResolvedNil(t *testing.T) {
	result, err := parseArgs([]string{"comment", "edit", "abc"})
	require.NoError(t, err)
	assert.Nil(t, result.commentResolved)
}

func TestParseArgs_CommentResolvedOnlyValidForEdit(t *testing.T) {
	_, err := parseArgs([]string{"comment", "list", "--resolved=true"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--resolved is only valid for comment edit")
}

func TestParseArgs_CommentResolvedWithResolveSubcommand(t *testing.T) {
	_, err := parseArgs([]string{"comment", "resolve", "abc", "--resolved=true"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--resolved cannot be combined with comment resolve")
}

func TestParseArgs_CommentResolvedWithUnresolveSubcommand(t *testing.T) {
	_, err := parseArgs([]string{"comment", "unresolve", "abc", "--resolved=false"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--resolved cannot be combined with comment unresolve")
}

func TestParseArgs_CommentResolvedInvalidValue(t *testing.T) {
	_, err := parseArgs([]string{"comment", "edit", "abc", "--resolved=maybe"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be true or false")
}

func TestParseArgs_CommentEditMissingID(t *testing.T) {
	_, err := parseArgs([]string{"comment", "edit"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires a comment ID")
}

func TestParseArgs_CommentResolveSubcommand(t *testing.T) {
	result, err := parseArgs([]string{"comment", "resolve", "abc123"})
	require.NoError(t, err)
	assert.Equal(t, "comment", result.cmd)
	assert.Equal(t, "resolve", result.commentSub)
	assert.Equal(t, "abc123", result.commentID)
}

func TestParseArgs_CommentUnresolveSubcommand(t *testing.T) {
	result, err := parseArgs([]string{"comment", "unresolve", "abc123"})
	require.NoError(t, err)
	assert.Equal(t, "comment", result.cmd)
	assert.Equal(t, "unresolve", result.commentSub)
	assert.Equal(t, "abc123", result.commentID)
}

func TestParseArgs_CommentResolveMissingID(t *testing.T) {
	_, err := parseArgs([]string{"comment", "resolve"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires a comment ID")
}

func TestParseArgs_CommentUnresolveMissingID(t *testing.T) {
	_, err := parseArgs([]string{"comment", "unresolve"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires a comment ID")
}

func TestParseArgs_CommentWithDiffFlags(t *testing.T) {
	_, err := parseArgs([]string{"comment", "--cached"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not accept diff/log/status flags")
}

func TestParseArgs_CommentVerbose(t *testing.T) {
	result, err := parseArgs([]string{"comment", "list", "-v"})
	require.NoError(t, err)
	assert.True(t, result.commentVerbose)
}

func TestParseArgs_CommentVerboseLong(t *testing.T) {
	result, err := parseArgs([]string{"comment", "list", "--verbose"})
	require.NoError(t, err)
	assert.True(t, result.commentVerbose)
}

func TestParseArgs_CommentStatusOnDiff(t *testing.T) {
	_, err := parseArgs([]string{"diff", "--status", "all"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only valid for comment")
}

func TestParseArgs_CommentAllBranches(t *testing.T) {
	result, err := parseArgs([]string{"comment", "list", "--all-branches"})
	require.NoError(t, err)
	assert.True(t, result.commentAllBranches)
}

func TestParseArgs_CommentAllBranchesOnDiff(t *testing.T) {
	_, err := parseArgs([]string{"diff", "--all-branches"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only valid for comment")
}

func TestParseArgs_CommentKind(t *testing.T) {
	for _, kind := range []string{"comment", "note", "all"} {
		result, err := parseArgs([]string{"comment", "list", "--kind", kind})
		require.NoError(t, err)
		assert.Equal(t, kind, result.commentKind)
	}
	// --kind=value form
	result, err := parseArgs([]string{"comment", "list", "--kind=note"})
	require.NoError(t, err)
	assert.Equal(t, "note", result.commentKind)
}

func TestParseArgs_CommentKindInvalid(t *testing.T) {
	_, err := parseArgs([]string{"comment", "list", "--kind", "bad"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "comment, note, or all")
}

func TestParseArgs_CommentKindOnDiff(t *testing.T) {
	_, err := parseArgs([]string{"diff", "--kind", "comment"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only valid for comment")
}

func TestParseArgs_CommentKindOnlyForList(t *testing.T) {
	_, err := parseArgs([]string{"comment", "edit", "123", "--kind", "comment"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only valid for comment list")
}

func TestParseArgs_NoteList(t *testing.T) {
	result, err := parseArgs([]string{"note", "list"})
	require.NoError(t, err)
	assert.Equal(t, "comment", result.cmd)
	assert.Equal(t, "note", result.cmdAlias)
	assert.Equal(t, "list", result.commentSub)
}

func TestParseArgs_NoteAlias(t *testing.T) {
	result, err := parseArgs([]string{"n", "list"})
	require.NoError(t, err)
	assert.Equal(t, "comment", result.cmd)
	assert.Equal(t, "note", result.cmdAlias)
}

func TestParseArgs_NoteAddNoFile(t *testing.T) {
	result, err := parseArgs([]string{"note", "add", "-m", "text"})
	require.NoError(t, err)
	assert.Equal(t, "comment", result.cmd)
	assert.Equal(t, "note", result.cmdAlias)
	assert.Equal(t, "add", result.commentSub)
	assert.Equal(t, "", result.commentAddTarget)
}

func TestParseArgs_NoteAddRejectsFile(t *testing.T) {
	_, err := parseArgs([]string{"note", "add", "foo.go:10", "-m", "text"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "note add does not accept a file:line argument")
}

func TestParseArgs_NoteEdit(t *testing.T) {
	result, err := parseArgs([]string{"note", "edit", "123"})
	require.NoError(t, err)
	assert.Equal(t, "comment", result.cmd)
	assert.Equal(t, "note", result.cmdAlias)
	assert.Equal(t, "edit", result.commentSub)
	assert.Equal(t, "123", result.commentID)
}

func TestParseArgs_NoteDefault(t *testing.T) {
	// No sub-subcommand defaults to list
	result, err := parseArgs([]string{"note"})
	require.NoError(t, err)
	assert.Equal(t, "comment", result.cmd)
	assert.Equal(t, "note", result.cmdAlias)
	assert.Equal(t, "list", result.commentSub)
}

func TestParseArgs_NoteListFlags(t *testing.T) {
	// Shared flags should work with note command
	result, err := parseArgs([]string{"note", "list", "--since", "2w", "--status", "all", "-v"})
	require.NoError(t, err)
	assert.Equal(t, "comment", result.cmd)
	assert.Equal(t, "note", result.cmdAlias)
	assert.Equal(t, "2w", result.since)
	assert.Equal(t, "all", result.commentStatus)
	assert.True(t, result.commentVerbose)
}

func TestParseArgs_NoteResolve(t *testing.T) {
	result, err := parseArgs([]string{"note", "resolve", "abc"})
	require.NoError(t, err)
	assert.Equal(t, "comment", result.cmd)
	assert.Equal(t, "note", result.cmdAlias)
	assert.Equal(t, "resolve", result.commentSub)
	assert.Equal(t, "abc", result.commentID)
}

func TestParseArgs_NoteUnresolve(t *testing.T) {
	result, err := parseArgs([]string{"note", "unresolve", "abc"})
	require.NoError(t, err)
	assert.Equal(t, "comment", result.cmd)
	assert.Equal(t, "note", result.cmdAlias)
	assert.Equal(t, "unresolve", result.commentSub)
	assert.Equal(t, "abc", result.commentID)
}

func TestParseArgs_NoteRejectsKind(t *testing.T) {
	_, err := parseArgs([]string{"note", "list", "--kind", "all"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--kind cannot be used with note")
}

func TestParseArgs_NoteN(t *testing.T) {
	result, err := parseArgs([]string{"note", "list", "-n1"})
	require.NoError(t, err)
	assert.Equal(t, 1, result.commentN)
	assert.True(t, result.commentNSet)
}

func TestParseArgs_CommentSince(t *testing.T) {
	result, err := parseArgs([]string{"comment", "list", "--since=2w"})
	require.NoError(t, err)
	assert.Equal(t, "comment", result.cmd)
	assert.Equal(t, "2w", result.since)
}

func TestParseArgs_CommentBranch(t *testing.T) {
	result, err := parseArgs([]string{"comment", "list", "--branch=main"})
	require.NoError(t, err)
	assert.Equal(t, "main", result.commentBranch)

	result, err = parseArgs([]string{"comment", "list", "--branch", "feature/x"})
	require.NoError(t, err)
	assert.Equal(t, "feature/x", result.commentBranch)

	// -b with explicit branch name
	result, err = parseArgs([]string{"comment", "list", "-b", "develop"})
	require.NoError(t, err)
	assert.Equal(t, "develop", result.commentBranch)

	// --branch with no arg means all branches
	result, err = parseArgs([]string{"comment", "list", "--branch"})
	require.NoError(t, err)
	assert.True(t, result.commentAllBranches)
	assert.Empty(t, result.commentBranch)

	// -b with no arg means all branches
	result, err = parseArgs([]string{"comment", "list", "-b"})
	require.NoError(t, err)
	assert.True(t, result.commentAllBranches)
	assert.Empty(t, result.commentBranch)

	// -b in status context means --branches, not --branch
	result, err = parseArgs([]string{"status", "-b"})
	require.NoError(t, err)
	assert.True(t, result.showBranches)
	assert.Empty(t, result.commentBranch)
}

func TestParseArgs_CommentBranchAllBranchesConflict(t *testing.T) {
	_, err := parseArgs([]string{"comment", "list", "--branch=main", "--all-branches"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be used together")

	// bare -b (all branches) followed by --branch=main also conflicts
	_, err = parseArgs([]string{"comment", "list", "-b", "--branch=main"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be used together")
}

func TestParseArgs_HelpComment(t *testing.T) {
	result, err := parseArgs([]string{"help", "comment"})
	require.NoError(t, err)
	assert.True(t, result.showHelp)
	assert.Equal(t, "comment", result.helpCmd)
}

func TestParseArgs_CommentHelp(t *testing.T) {
	result, err := parseArgs([]string{"comment", "--help"})
	require.NoError(t, err)
	assert.True(t, result.showHelp)
	assert.Equal(t, "comment", result.helpCmd)
}

func TestParseArgs_CommentAdd(t *testing.T) {
	result, err := parseArgs([]string{"comment", "add", "main.go:42", "-m", "Fix this"})
	require.NoError(t, err)
	assert.Equal(t, "comment", result.cmd)
	assert.Equal(t, "add", result.commentSub)
	assert.Equal(t, "main.go:42", result.commentAddTarget)
	assert.Equal(t, "Fix this", result.commentAddMessage)
}

func TestParseArgs_CommentAddRef(t *testing.T) {
	result, err := parseArgs([]string{"comment", "add", "src/app.go:10", "-m", "Note", "--ref", "main"})
	require.NoError(t, err)
	assert.Equal(t, "add", result.commentSub)
	assert.Equal(t, "src/app.go:10", result.commentAddTarget)
	assert.Equal(t, "Note", result.commentAddMessage)
	assert.Equal(t, "main", result.commentAddRef)
}

func TestParseArgs_CommentAddRefEquals(t *testing.T) {
	result, err := parseArgs([]string{"comment", "add", "f.go:1", "-m", "x", "--ref=abc123"})
	require.NoError(t, err)
	assert.Equal(t, "abc123", result.commentAddRef)
}

func TestParseArgs_CommentAddStandalone(t *testing.T) {
	// No file:line is valid — creates a standalone comment
	args, err := parseArgs([]string{"comment", "add", "-m", "text"})
	require.NoError(t, err)
	assert.Equal(t, "add", args.commentSub)
	assert.Equal(t, "", args.commentAddTarget)
	assert.Equal(t, "text", args.commentAddMessage)
}

func TestParseArgs_CommentAddMOnNonAdd(t *testing.T) {
	_, err := parseArgs([]string{"comment", "list", "-m", "text"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "-m is only valid for comment add")
}

func TestParseArgs_CommentAddRefOnNonAdd(t *testing.T) {
	_, err := parseArgs([]string{"comment", "list", "--ref", "main"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--ref is only valid for comment add")
}

func TestParseArgs_CommentAddAttachedM(t *testing.T) {
	result, err := parseArgs([]string{"comment", "add", "f.go:1", "-mhello"})
	require.NoError(t, err)
	assert.Equal(t, "hello", result.commentAddMessage)
}

func TestParseArgs_CommentAddAuthor(t *testing.T) {
	result, err := parseArgs([]string{"comment", "add", "main.go:42", "-m", "Note", "--author", "Claude"})
	require.NoError(t, err)
	assert.Equal(t, "add", result.commentSub)
	assert.Equal(t, "Claude", result.commentAuthor)
}

func TestParseArgs_CommentAddAuthorEquals(t *testing.T) {
	result, err := parseArgs([]string{"comment", "add", "f.go:1", "-m", "x", "--author=Bot"})
	require.NoError(t, err)
	assert.Equal(t, "Bot", result.commentAuthor)
}

func TestParseArgs_CommentAuthorOnList(t *testing.T) {
	result, err := parseArgs([]string{"comment", "list", "--author", "Claude"})
	require.NoError(t, err)
	assert.Equal(t, "list", result.commentSub)
	assert.Equal(t, "Claude", result.commentAuthor)
}

func TestParseArgs_CommentAuthorOnEdit(t *testing.T) {
	_, err := parseArgs([]string{"comment", "edit", "abc123", "--author", "Claude"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--author is only valid for")
}

func TestParseArgs_CommentAddBareAuthor(t *testing.T) {
	_, err := parseArgs([]string{"comment", "add", "f.go:1", "-m", "x", "--author"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--author requires an author argument")
}

func TestParseArgs_CommentListBareAuthor(t *testing.T) {
	result, err := parseArgs([]string{"comment", "list", "--author"})
	require.NoError(t, err)
	assert.True(t, result.commentAuthorSet)
	assert.Equal(t, "", result.commentAuthor)
}

func TestParseArgs_CommentAddAuthorEmptyEquals(t *testing.T) {
	_, err := parseArgs([]string{"comment", "add", "f.go:1", "-m", "x", "--author="})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--author requires a value when using = syntax")
}

func TestParseCommentTarget(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		wantFile string
		wantLine int
		wantErr  bool
	}{
		{"simple", "main.go:42", "main.go", 42, false},
		{"path with dirs", "src/pkg/file.go:100", "src/pkg/file.go", 100, false},
		{"line 1", "f.go:1", "f.go", 1, false},
		{"no colon", "main.go", "", 0, true},
		{"no line number", "main.go:", "", 0, true},
		{"zero line", "main.go:0", "", 0, true},
		{"negative line", "main.go:-1", "", 0, true},
		{"non-numeric line", "main.go:abc", "", 0, true},
		{"empty file", ":42", "", 0, true},
		{"colon in path", "src/a:b/file.go:10", "src/a:b/file.go", 10, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, line, err := parseCommentTarget(tt.target)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantFile, file)
				assert.Equal(t, tt.wantLine, line)
			}
		})
	}
}

func TestIsLineInDiff(t *testing.T) {
	diffOutput := `diff --git a/main.go b/main.go
index abc..def 100644
--- a/main.go
+++ b/main.go
@@ -10,6 +10,7 @@ func main() {
 	existing := 1
 	more := 2
 	stuff := 3
+	newLine := 4
 	after := 5
 	end := 6
 }
`
	// Line 13 is the added line (newLine := 4)
	assert.True(t, isLineInDiff(diffOutput, "main.go", 13))
	// Line 10 is context, not changed
	assert.False(t, isLineInDiff(diffOutput, "main.go", 10))
	// Different file
	assert.False(t, isLineInDiff(diffOutput, "other.go", 13))
	// Empty diff
	assert.False(t, isLineInDiff("", "main.go", 1))
}

// testCommentStyles returns default CommentListStyles for use in tests.
func testCommentStyles() tui.CommentListStyles {
	tui.ApplyTheme(config.ThemeConfig{})
	return tui.CommentListTheme()
}

// testOneline is a helper that computes cols from a single comment and formats it.
func testOneline(c *comments.Comment, displayID string, termWidth int, now time.Time) string {
	ids := map[string]string{}
	if displayID != "" {
		ids[c.ID] = displayID
	}
	cols := computeOnelineCols([]*comments.Comment{c}, ids, now)
	return formatCommentOneline(c, displayID, termWidth, now, cols, testCommentStyles())
}

func TestFormatCommentOneline(t *testing.T) {
	tests := []struct {
		name      string
		comment   *comments.Comment
		termWidth int
		contains  []string
	}{
		{
			name: "basic",
			comment: &comments.Comment{
				ID:        "1705312200000",
				File:      "src/foo.go",
				Line:      42,
				CommitSHA: "abc123def456",
				Text:      "Fix this bug",
			},
			termWidth: 120,
			contains:  []string{"17053122", "src/foo.go:42", "abc123d", "Fix this bug"},
		},
		{
			name: "resolved",
			comment: &comments.Comment{
				ID:        "100",
				File:      "test.go",
				Line:      1,
				CommitSHA: "abc1234",
				Resolved:  true,
				Text:      "Done",
			},
			termWidth: 120,
			contains:  []string{"Done"},
		},
		{
			name: "no commit",
			comment: &comments.Comment{
				ID:   "101",
				File: "test.go",
				Line: 1,
				Text: "No commit",
			},
			termWidth: 120,
			contains:  []string{" - "},
		},
		{
			name: "long text truncated",
			comment: &comments.Comment{
				ID:        "102",
				File:      "test.go",
				Line:      1,
				CommitSHA: "abc1234",
				Text:      "This is a very long comment that should be truncated after sixty characters total",
			},
			termWidth: 80,
			contains:  []string{"..."},
		},
		{
			name: "multiline uses first line",
			comment: &comments.Comment{
				ID:        "103",
				File:      "test.go",
				Line:      1,
				CommitSHA: "abc1234",
				Text:      "First line\nSecond line\nThird line",
			},
			termWidth: 120,
			contains:  []string{"First line"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line := stripANSI(testOneline(tt.comment, "", tt.termWidth, time.Now()))
			for _, s := range tt.contains {
				assert.Contains(t, line, s)
			}
		})
	}
}

func TestFormatCommentOneline_Standalone(t *testing.T) {
	c := &comments.Comment{
		ID:        "200",
		CommitSHA: "abc1234",
		Text:      "A general note",
	}
	line := stripANSI(testOneline(c, "", 120, time.Now()))
	assert.NotContains(t, line, "(standalone)")
	assert.Contains(t, line, "A general note")
}

func TestFormatCommentOneline_ResolvedStyling(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	c := &comments.Comment{
		ID:        "400",
		File:      "test.go",
		Line:      1,
		CommitSHA: "abc1234",
		Branch:    "main",
		Resolved:  true,
		Text:      "Done with this",
	}
	raw := testOneline(c, "", 120, time.Now())
	// Resolved columns should have strikethrough ANSI sequences (;9m or [9m)
	assert.Regexp(t, `\x1b\[[0-9;]*9m`, raw, "expected strikethrough ANSI sequences for resolved comment")
	// Text should be styled (not plain)
	plain := stripANSI(raw)
	assert.Contains(t, plain, "Done with this")
	assert.Greater(t, len(raw), len(plain), "resolved text should have ANSI styling")
	// No [resolved] tag
	assert.NotContains(t, plain, "[resolved]")
}

func TestFormatCommentOneline_DateColumn(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	c := &comments.Comment{
		ID:        "300",
		File:      "test.go",
		Line:      1,
		CommitSHA: "abc1234",
		Created:   time.Date(2026, 3, 10, 14, 30, 0, 0, time.UTC),
		Text:      "Date test",
	}
	line := stripANSI(testOneline(c, "", 120, now))
	assert.Contains(t, line, "Mar 10 14:30")
	assert.Contains(t, line, "4d")
}

func TestFormatCommentOneline_MultilineExcludesSecond(t *testing.T) {
	c := &comments.Comment{
		ID:        "103",
		File:      "test.go",
		Line:      1,
		CommitSHA: "abc1234",
		Text:      "First line\nSecond line",
	}
	line := stripANSI(testOneline(c, "", 120, time.Now()))
	assert.NotContains(t, line, "Second line")
}

func TestShortSuffixes(t *testing.T) {
	tests := []struct {
		name string
		ids  []string
		want map[string]string
	}{
		{
			name: "single ID",
			ids:  []string{"1770968997415"},
			want: map[string]string{"1770968997415": "415"},
		},
		{
			name: "two distinct",
			ids:  []string{"1770968997415", "1770881758352"},
			want: map[string]string{"1770968997415": "415", "1770881758352": "352"},
		},
		{
			name: "need longer suffix",
			ids:  []string{"1770968997415", "1770881757415"},
			want: map[string]string{"1770968997415": "97415", "1770881757415": "57415"},
		},
		{
			name: "empty",
			ids:  []string{},
			want: map[string]string{},
		},
		{
			name: "per-ID suffix lengths",
			ids:  []string{"1770968997415", "1770881757415", "1770000000000"},
			want: map[string]string{
				"1770968997415": "97415",
				"1770881757415": "57415",
				"1770000000000": "000",
			},
		},
		{
			name: "differ only in first char",
			ids:  []string{"xabc", "yabc"},
			want: map[string]string{"xabc": "xabc", "yabc": "yabc"},
		},
		{
			name: "short IDs",
			ids:  []string{"abc", "def"},
			want: map[string]string{"abc": "abc", "def": "def"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortSuffixes(tt.ids)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatCommentOneline_ShortID(t *testing.T) {
	c := &comments.Comment{
		ID:        "1770968997415",
		File:      "test.go",
		Line:      1,
		CommitSHA: "abc1234",
		Text:      "Hello",
	}
	line := stripANSI(testOneline(c, "7415", 120, time.Now()))
	assert.Contains(t, line, "7415")
	assert.NotContains(t, line, "1770968997415")
}

func TestFormatCommentBlock(t *testing.T) {
	created := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	c := &comments.Comment{
		ID:        "1705312200000",
		File:      "src/foo.go",
		Line:      42,
		CommitSHA: "abc123def456",
		Branch:    "main",
		Created:   created,
		Text:      "Fix this bug\nIt causes crashes",
		Context: comments.LineContext{
			Above: []string{"func foo() {", "    x := 1"},
			Line:  "    return x",
			Below: []string{"}"},
		},
	}

	block := stripANSI(formatCommentBlock(c, nil, 120, "", c.Created, testCommentStyles()))

	// Metadata (two-column layout at width 120)
	assert.Contains(t, block, "┃ Date:   Jan 15 10:30 0m")
	assert.Contains(t, block, "File:   src/foo.go:42\n")
	assert.Contains(t, block, "┃ Status: unresolved")
	assert.Contains(t, block, "Ref:    abc123d on main\n")
	assert.Contains(t, block, "┃ ID:     1705312200000\n")
	// Diff context (line numbers: 40-43, gutter width 2)
	assert.Contains(t, block, "┃   40 func foo() {\n")
	assert.Contains(t, block, "┃   41     x := 1\n")
	assert.Contains(t, block, "┃ > 42     return x\n")
	assert.Contains(t, block, "┃   43 }\n")
	// Comment text
	assert.Contains(t, block, "┃ Fix this bug\n")
	assert.Contains(t, block, "┃ It causes crashes\n")
}

func TestFormatCommentBlock_NarrowTerminal(t *testing.T) {
	created := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	c := &comments.Comment{
		ID:        "1705312200000",
		File:      "src/foo.go",
		Line:      42,
		CommitSHA: "abc123def456",
		Branch:    "main",
		Created:   created,
		Text:      "Fix this bug",
		Context: comments.LineContext{
			Above: []string{"func foo() {"},
			Line:  "    return x",
			Below: []string{"}"},
		},
	}

	// Narrow terminal forces single-column layout
	block := stripANSI(formatCommentBlock(c, nil, 40, "", c.Created, testCommentStyles()))

	// Each field on its own line
	assert.Contains(t, block, "┃ Date:   Jan 15 10:30 0m\n")
	assert.Contains(t, block, "┃ Status: unresolved\n")
	assert.Contains(t, block, "┃ ID:     1705312200000\n")
	assert.Contains(t, block, "┃ File:   src/foo.go:42\n")
	assert.Contains(t, block, "┃ Ref:    abc123d on main\n")
}

func TestFormatCommentBlock_Resolved(t *testing.T) {
	c := &comments.Comment{
		ID:       "100",
		File:     "test.go",
		Line:     1,
		Created:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Resolved: true,
		Text:     "Done",
		Context:  comments.LineContext{Line: "code"},
	}
	block := stripANSI(formatCommentBlock(c, nil, 120, "", c.Created, testCommentStyles()))
	assert.Contains(t, block, "┃ Status: resolved\n")
	assert.Contains(t, block, "┃ ID:     100\n")
}

func TestFormatCommentBlock_NoCommit(t *testing.T) {
	c := &comments.Comment{
		ID:      "101",
		File:    "test.go",
		Line:    1,
		Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Text:    "No commit",
		Context: comments.LineContext{Line: "code"},
	}
	block := stripANSI(formatCommentBlock(c, nil, 120, "", c.Created, testCommentStyles()))
	assert.NotContains(t, block, "Ref:")
}

func TestFormatCommentBlock_Highlighted(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	h := highlight.New()
	defer h.Close()

	c := &comments.Comment{
		ID:      "200",
		File:    "test.go",
		Line:    3,
		Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Text:    "Check this",
		Context: comments.LineContext{
			Above: []string{"func foo() {", "    x := 1"},
			Line:  "    return x",
			Below: []string{"}"},
		},
	}

	// With highlighter: should produce valid output (ANSI codes present)
	block := formatCommentBlock(c, h, 120, "", c.Created, testCommentStyles())
	stripped := stripANSI(block)

	// Content should be the same after stripping ANSI
	assert.Contains(t, stripped, "func foo() {\n")
	assert.Contains(t, stripped, "    return x\n")

	// The highlighted version should have ANSI codes (more bytes than stripped)
	assert.Greater(t, len(block), len(stripped), "highlighting should add ANSI codes")

	// Nil highlighter should also work (plain text)
	plain := formatCommentBlock(c, nil, 120, "", c.Created, testCommentStyles())
	plainStripped := stripANSI(plain)
	assert.Equal(t, stripped, plainStripped, "stripped output should match regardless of highlighter")
}

func TestFormatCommentBlock_UnsupportedLanguage(t *testing.T) {
	h := highlight.New()
	defer h.Close()

	c := &comments.Comment{
		ID:      "201",
		File:    "data.xyz",
		Line:    1,
		Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Text:    "Unknown file type",
		Context: comments.LineContext{Line: "some content"},
	}

	// Should gracefully fall back to plain text
	block := formatCommentBlock(c, h, 120, "", c.Created, testCommentStyles())
	stripped := stripANSI(block)
	assert.Contains(t, stripped, "some content")
}

func TestFormatCommentOneline_WithAuthor(t *testing.T) {
	c := &comments.Comment{
		ID:        "200",
		File:      "test.go",
		Line:      1,
		CommitSHA: "abc1234",
		Author:    "Claude",
		Text:      "Review note",
	}
	line := stripANSI(testOneline(c, "", 120, time.Now()))
	assert.Contains(t, line, "[Claude]")
	assert.Contains(t, line, "Review note")
}

func TestFormatCommentOneline_WithoutAuthor(t *testing.T) {
	c := &comments.Comment{
		ID:        "201",
		File:      "test.go",
		Line:      1,
		CommitSHA: "abc1234",
		Text:      "Human note",
	}
	line := stripANSI(testOneline(c, "", 120, time.Now()))
	assert.NotContains(t, line, "[")
	assert.Contains(t, line, "Human note")
}

func TestFormatCommentBlock_WithAuthor(t *testing.T) {
	c := &comments.Comment{
		ID:      "300",
		File:    "test.go",
		Line:    1,
		Branch:  "main",
		Author:  "Claude",
		Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Text:    "Check this",
		Context: comments.LineContext{Line: "code"},
	}
	block := stripANSI(formatCommentBlock(c, nil, 120, "", c.Created, testCommentStyles()))
	// Author should appear in the right column as a header
	assert.Contains(t, block, "Author: Claude")
	// Should NOT appear as a separate "commented" header line
	assert.NotContains(t, block, "Claude commented")
}

func TestFormatCommentBlock_WithoutAuthor(t *testing.T) {
	c := &comments.Comment{
		ID:      "301",
		File:    "test.go",
		Line:    1,
		Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Text:    "No author",
		Context: comments.LineContext{Line: "code"},
	}
	block := stripANSI(formatCommentBlock(c, nil, 120, "", c.Created, testCommentStyles()))
	assert.NotContains(t, block, "Author:")
}

func TestFormatCommentBlock_AuthorNarrowTerminal(t *testing.T) {
	c := &comments.Comment{
		ID:      "302",
		File:    "test.go",
		Line:    1,
		Author:  "Bot",
		Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Text:    "Note",
		Context: comments.LineContext{Line: "code"},
	}
	// Single column fallback
	block := stripANSI(formatCommentBlock(c, nil, 40, "", c.Created, testCommentStyles()))
	assert.Contains(t, block, "Author: Bot")
}
