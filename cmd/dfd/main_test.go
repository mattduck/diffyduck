package main

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/content"
)

func TestParseArgs_Empty(t *testing.T) {
	res, err := parseArgs([]string{})
	require.NoError(t, err)
	assert.Equal(t, "diff", res.cmd)
	assert.Empty(t, res.refs)
	assert.Empty(t, res.paths)
	assert.Equal(t, content.ModeDiffUnstaged, res.mode)
}

func TestParseArgs_SubcommandDetection(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantCmd string
	}{
		{"diff explicit", []string{"diff"}, "diff"},
		{"diff alias", []string{"d"}, "diff"},
		{"diff default", []string{"HEAD"}, "diff"},
		{"show", []string{"show"}, "show"},
		{"log", []string{"log"}, "log"},
		{"log alias", []string{"l"}, "log"},
		{"clean", []string{"clean"}, "clean"},
		{"branches", []string{"branches"}, "branches"},
		{"branches alias", []string{"b"}, "branches"},
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

func TestParseArgs_BranchesWithArgs(t *testing.T) {
	_, err := parseArgs([]string{"branches", "main"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not accept arguments")
}

func TestParseArgs_BranchesWithFlags(t *testing.T) {
	_, err := parseArgs([]string{"branches", "--cached"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only accepts -v/--verbose and --since")
}

func TestParseArgs_BranchesVerbose(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"-v", []string{"branches", "-v"}},
		{"--verbose", []string{"branches", "--verbose"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseArgs(tt.args)
			require.NoError(t, err)
			assert.Equal(t, "branches", result.cmd)
			assert.True(t, result.verbose)
		})
	}
}

func TestParseArgs_BranchesSince(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		since string
	}{
		{"--since=7d", []string{"branches", "--since=7d"}, "7d"},
		{"--since 2w", []string{"branches", "--since", "2w"}, "2w"},
		{"--since=all", []string{"branches", "--since=all"}, "all"},
		{"--since=1y", []string{"branches", "--since=1y"}, "1y"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseArgs(tt.args)
			require.NoError(t, err)
			assert.Equal(t, "branches", result.cmd)
			assert.Equal(t, tt.since, result.since)
		})
	}
}

func TestParseArgs_SinceOnDiff(t *testing.T) {
	_, err := parseArgs([]string{"diff", "--since=7d"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only valid for branches")
}

func TestParseArgs_SinceMissingValue(t *testing.T) {
	_, err := parseArgs([]string{"branches", "--since"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires a duration")
}

func TestParseSinceDuration(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
		err   bool
	}{
		{"7d", 7 * 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"3m", 90 * 24 * time.Hour, false},
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
	assert.Contains(t, err.Error(), "only valid for branches")
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
		{"b --help", []string{"b", "--help"}, "branches"},
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
		{"help branches", []string{"help", "branches"}, "branches"},
		{"help b", []string{"help", "b"}, "branches"},
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
	assert.Contains(t, output, "branches, b")
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
