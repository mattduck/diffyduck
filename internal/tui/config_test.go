package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/user/diffyduck/pkg/config"
	"github.com/user/diffyduck/pkg/highlight"
)

func TestApplyKeysConfig_EmptyConfig(t *testing.T) {
	km := ApplyKeysConfig(config.KeysConfig{})
	assert.Equal(t, DefaultKeyMap(), km)
}

func TestApplyKeysConfig_SingleOverride(t *testing.T) {
	cfg := config.KeysConfig{
		Navigation: &config.NavigationKeys{
			Up: []string{"w"},
		},
	}
	km := ApplyKeysConfig(cfg)

	assert.Equal(t, []string{"w"}, km.Up)
	// Other actions unchanged
	assert.Equal(t, DefaultKeyMap().Down, km.Down)
	assert.Equal(t, DefaultKeyMap().Quit, km.Quit)
}

func TestApplyKeysConfig_MultipleSubsections(t *testing.T) {
	cfg := config.KeysConfig{
		Navigation: &config.NavigationKeys{
			Up:   []string{"w"},
			Down: []string{"s"},
		},
		Actions: &config.ActionKeys{
			Quit: []string{"Z", "Q"},
		},
	}
	km := ApplyKeysConfig(cfg)

	assert.Equal(t, []string{"w"}, km.Up)
	assert.Equal(t, []string{"s"}, km.Down)
	assert.Equal(t, []string{"Z", "Q"}, km.Quit)
	// Unchanged
	assert.Equal(t, DefaultKeyMap().PageUp, km.PageUp)
}

func TestApplyKeysConfig_NarrowToggleIndependent(t *testing.T) {
	cfg := config.KeysConfig{
		Search: &config.SearchKeys{
			PrevMatch:    []string{"P"},
			NarrowToggle: []string{"space n"},
		},
	}
	km := ApplyKeysConfig(cfg)

	assert.Equal(t, []string{"P"}, km.PrevMatch)
	assert.Equal(t, []string{"space n"}, km.NarrowToggle, "NarrowToggle should be independent of PrevMatch")
}

func TestApplyKeysConfig_NilSubsectionKeepsDefaults(t *testing.T) {
	// Only set navigation, all other subsections nil
	cfg := config.KeysConfig{
		Navigation: &config.NavigationKeys{
			Up: []string{"w"},
		},
	}
	km := ApplyKeysConfig(cfg)

	// Navigation changed
	assert.Equal(t, []string{"w"}, km.Up)
	// All other subsections unchanged
	assert.Equal(t, DefaultKeyMap().Quit, km.Quit)
	assert.Equal(t, DefaultKeyMap().SearchForward, km.SearchForward)
	assert.Equal(t, DefaultKeyMap().FoldToggle, km.FoldToggle)
	assert.Equal(t, DefaultKeyMap().Help, km.Help)
	assert.Equal(t, DefaultKeyMap().WinSplitV, km.WinSplitV)
	assert.Equal(t, DefaultKeyMap().VisualExit, km.VisualExit)
}

func TestApplyKeysConfig_SequenceBindings(t *testing.T) {
	cfg := config.KeysConfig{
		Navigation: &config.NavigationKeys{
			GoToTop:     []string{"z z"},
			NextHeading: []string{"z j"},
			PrevHeading: []string{"z k"},
		},
	}
	km := ApplyKeysConfig(cfg)

	assert.Equal(t, []string{"z z"}, km.GoToTop)
	assert.Equal(t, []string{"z j"}, km.NextHeading)
	assert.Equal(t, []string{"z k"}, km.PrevHeading)
}

func TestApplyKeysConfig_ChangeNavigation(t *testing.T) {
	cfg := config.KeysConfig{
		Navigation: &config.NavigationKeys{
			NextChange: []string{"space d j"},
			PrevChange: []string{"space d k"},
		},
	}
	km := ApplyKeysConfig(cfg)

	assert.Equal(t, []string{"space d j"}, km.NextChange)
	assert.Equal(t, []string{"space d k"}, km.PrevChange)
	// Other nav defaults unchanged
	assert.Equal(t, DefaultKeyMap().NextHeading, km.NextHeading)
}

func TestApplyKeysConfig_WindowBindings(t *testing.T) {
	cfg := config.KeysConfig{
		Window: &config.WindowKeys{
			SplitVertical: []string{"ctrl+w v"},
			Close:         []string{"ctrl+w c"},
		},
	}
	km := ApplyKeysConfig(cfg)

	assert.Equal(t, []string{"ctrl+w v"}, km.WinSplitV)
	assert.Equal(t, []string{"ctrl+w c"}, km.WinClose)
	// Unchanged
	assert.Equal(t, DefaultKeyMap().WinSplitH, km.WinSplitH)
	assert.Equal(t, DefaultKeyMap().WinFocusLeft, km.WinFocusLeft)
}

func TestApplyKeysConfig_VisualBindings(t *testing.T) {
	cfg := config.KeysConfig{
		Visual: &config.VisualKeys{
			Exit: []string{"q"},
		},
	}
	km := ApplyKeysConfig(cfg)

	assert.Equal(t, []string{"q"}, km.VisualExit)
}

func TestApplyKeysConfig_BuildsPrefixSet(t *testing.T) {
	km := ApplyKeysConfig(config.KeysConfig{})

	assert.True(t, km.prefixSet["g"], "g should be a prefix (from g g, g j, g k)")
	assert.True(t, km.prefixSet["ctrl+w"], "ctrl+w should be a prefix (from window bindings)")
	assert.False(t, km.prefixSet["q"], "q should not be a prefix")
}

func TestApplyKeysConfig_CustomPrefixSet(t *testing.T) {
	cfg := config.KeysConfig{
		Navigation: &config.NavigationKeys{
			GoToTop: []string{"z z"},
		},
	}
	km := ApplyKeysConfig(cfg)

	assert.True(t, km.prefixSet["z"], "z should be a prefix (from z z)")
	assert.True(t, km.prefixSet["g"], "g still a prefix (from g j, g k defaults)")
}

func TestValidateBindings_NoConflict(t *testing.T) {
	km := DefaultKeyMap()
	err := ValidateBindings(km)
	assert.NoError(t, err)
}

func TestValidateBindings_Conflict(t *testing.T) {
	km := DefaultKeyMap()
	// Add "g" as a direct binding — conflicts with "g g" prefix
	km.Quit = []string{"g"}
	km.prefixSet = buildPrefixSet(km)

	err := ValidateBindings(km)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `"g"`)
}

func TestWithConfig_AppliesKeysAndFeatures(t *testing.T) {
	step := 12
	batch := 50
	cfg := config.Config{
		Keys: config.KeysConfig{
			Navigation: &config.NavigationKeys{
				Up: []string{"w"},
			},
		},
		Features: config.FeaturesConfig{
			HScrollStep:     &step,
			CommitBatchSize: &batch,
		},
	}

	m := New(nil, WithConfig(cfg))
	assert.Equal(t, []string{"w"}, m.keys.Up)
	assert.Equal(t, 12, m.hscrollStep)
	assert.Equal(t, 50, m.commitBatchSize)
}

func TestBuildHighlightTheme_Nil(t *testing.T) {
	theme := buildHighlightTheme(nil)
	defaults := highlight.DefaultTheme()

	// Should be identical to defaults
	assert.Equal(t, defaults.Style(highlight.CategoryKeyword), theme.Style(highlight.CategoryKeyword))
	assert.Equal(t, defaults.Style(highlight.CategoryString), theme.Style(highlight.CategoryString))
}

func TestBuildHighlightTheme_Overrides(t *testing.T) {
	cfg := &config.SyntaxConfig{
		Keyword:  "#ff0000",
		Function: "14",
	}
	theme := buildHighlightTheme(cfg)
	defaults := highlight.DefaultTheme()

	// Overridden categories should differ from defaults
	assert.NotEqual(t, defaults.Style(highlight.CategoryKeyword), theme.Style(highlight.CategoryKeyword))
	assert.NotEqual(t, defaults.Style(highlight.CategoryFunction), theme.Style(highlight.CategoryFunction))
	// FunctionCall should also be overridden (grouped with Function)
	assert.Equal(t, theme.Style(highlight.CategoryFunction), theme.Style(highlight.CategoryFunctionCall))
	// Unmentioned categories should keep defaults
	assert.Equal(t, defaults.Style(highlight.CategoryComment), theme.Style(highlight.CategoryComment))
}

func TestBuildHighlightTheme_NumberGrouping(t *testing.T) {
	cfg := &config.SyntaxConfig{
		Number: "196",
	}
	theme := buildHighlightTheme(cfg)

	// Number, Boolean, Nil, Constant should all have the same style
	numStyle := theme.Style(highlight.CategoryNumber)
	assert.Equal(t, numStyle, theme.Style(highlight.CategoryBoolean))
	assert.Equal(t, numStyle, theme.Style(highlight.CategoryNil))
	assert.Equal(t, numStyle, theme.Style(highlight.CategoryConstant))
}

func TestWithConfig_SyntaxTheme(t *testing.T) {
	cfg := config.Config{
		Theme: config.ThemeConfig{
			Syntax: &config.SyntaxConfig{
				Keyword: "#ff0000",
			},
		},
	}

	m := New(nil, WithConfig(cfg))
	defaults := highlight.DefaultTheme()

	// Highlighter should have the custom theme, not defaults
	assert.NotEqual(t, defaults.Style(highlight.CategoryKeyword), m.highlighter.Theme().Style(highlight.CategoryKeyword))
	// Unmentioned categories should still match defaults
	assert.Equal(t, defaults.Style(highlight.CategoryComment), m.highlighter.Theme().Style(highlight.CategoryComment))
}

func TestWithConfig_NoSyntaxKeepsDefault(t *testing.T) {
	cfg := config.Config{
		Theme: config.ThemeConfig{
			Added: "10", // only diff theme, no syntax
		},
	}

	m := New(nil, WithConfig(cfg))
	defaults := highlight.DefaultTheme()

	// Without [theme.syntax], highlighter should use defaults
	assert.Equal(t, defaults.Style(highlight.CategoryKeyword), m.highlighter.Theme().Style(highlight.CategoryKeyword))
}

func TestWithConfig_CLIFlagsWin(t *testing.T) {
	batch := 50
	cfg := config.Config{
		Features: config.FeaturesConfig{
			CommitBatchSize: &batch,
		},
	}

	// WithConfig first, then WithPagination (simulating CLI -n flag) — CLI wins
	m := New(nil, WithConfig(cfg), WithPagination(10, 200))
	assert.Equal(t, 200, m.commitBatchSize, "CLI flag should override config")
}
