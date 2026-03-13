package config

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sampleDefaultKeys returns a KeysConfig that mimics DefaultKeyMap(),
// duplicated here to avoid importing internal/tui.
func sampleDefaultKeys() KeysConfig {
	return KeysConfig{
		Navigation: &NavigationKeys{
			Up:          []string{"up", "k"},
			Down:        []string{"down", "j"},
			PageUp:      []string{"pgup", "ctrl+b", "b"},
			PageDown:    []string{"pgdown", "ctrl+f", " ", "f"},
			HalfUp:      []string{"ctrl+u", "u"},
			HalfDown:    []string{"ctrl+d", "d"},
			Top:         []string{"home"},
			Bottom:      []string{"end", "G"},
			Left:        []string{"left", "h"},
			Right:       []string{"right", "l"},
			GoToTop:     []string{"g g"},
			NextHeading: []string{"g j"},
			PrevHeading: []string{"g k"},
			NextComment: []string{"space c j"},
			PrevComment: []string{"space c k"},
			NextChange:  []string{"space g j"},
			PrevChange:  []string{"space g k"},
			NarrowNext:  []string{"ctrl+j"},
			NarrowPrev:  []string{"ctrl+k"},
		},
		Search: &SearchKeys{
			SearchFwd:    []string{"/"},
			SearchBack:   []string{"?"},
			NextMatch:    []string{"n"},
			PrevMatch:    []string{"N"},
			NarrowToggle: []string{"space n f"},
		},
		Folds: &FoldKeys{
			Fold:     []string{"tab"},
			FoldAll:  []string{"shift+tab"},
			FullFile: []string{"F"},
		},
		Actions: &ActionKeys{
			Quit:            []string{"q", "ctrl+c"},
			Enter:           []string{"enter"},
			Yank:            []string{"y"},
			YankUnresolved:  []string{"space c y"},
			YankAllComments: []string{"space c Y"},
			Refresh:         []string{"r"},
			Snapshot:        []string{"R"},
			Visual:          []string{"V"},
			Help:            []string{"ctrl+h"},
		},
		Window: &WindowKeys{
			SplitVertical:   []string{"ctrl+w %"},
			SplitHorizontal: []string{"ctrl+w \""},
			Close:           []string{"ctrl+w x"},
			FocusLeft:       []string{"ctrl+w h"},
			FocusRight:      []string{"ctrl+w l"},
			FocusUp:         []string{"ctrl+w k"},
			FocusDown:       []string{"ctrl+w j"},
			ResizeLeft:      []string{"ctrl+w ctrl+h"},
			ResizeRight:     []string{"ctrl+w ctrl+l"},
			ResizeUp:        []string{"ctrl+w ctrl+k"},
			ResizeDown:      []string{"ctrl+w ctrl+j"},
		},
		Visual: &VisualKeys{
			Exit: []string{"esc", "ctrl+g"},
		},
	}
}

func TestGenerateExample_ValidTOML(t *testing.T) {
	output := GenerateExample(sampleDefaultKeys())

	// Should parse as valid TOML (comments are stripped by the parser)
	var cfg Config
	_, err := toml.Decode(output, &cfg)
	require.NoError(t, err, "generated example must be valid TOML")
}

func TestGenerateExample_RoundTrip(t *testing.T) {
	// The generated example has all values commented out, so parsing it
	// should yield no active settings. TOML section headers like
	// [keys.navigation] create non-nil pointers, but all fields within
	// should be zero-valued.
	output := GenerateExample(sampleDefaultKeys())

	var cfg Config
	_, err := toml.Decode(output, &cfg)
	require.NoError(t, err)

	// Section headers may create non-nil subsection pointers,
	// but all field values should be nil (commented out).
	if cfg.Keys.Navigation != nil {
		assert.Equal(t, NavigationKeys{}, *cfg.Keys.Navigation)
	}
	if cfg.Keys.Search != nil {
		assert.Equal(t, SearchKeys{}, *cfg.Keys.Search)
	}
	if cfg.Keys.Folds != nil {
		assert.Equal(t, FoldKeys{}, *cfg.Keys.Folds)
	}
	if cfg.Keys.Actions != nil {
		assert.Equal(t, ActionKeys{}, *cfg.Keys.Actions)
	}
	if cfg.Keys.Window != nil {
		assert.Equal(t, WindowKeys{}, *cfg.Keys.Window)
	}
	if cfg.Keys.Visual != nil {
		assert.Equal(t, VisualKeys{}, *cfg.Keys.Visual)
	}
	// [theme.syntax] header creates a non-nil pointer, but all fields should be empty.
	if cfg.Theme.Syntax != nil {
		assert.Equal(t, SyntaxConfig{}, *cfg.Theme.Syntax)
	}
	cfg.Theme.Syntax = nil // zero out for comparison below
	assert.Equal(t, ThemeConfig{}, cfg.Theme, "all theme values should be commented out")
	assert.Equal(t, FeaturesConfig{}, cfg.Features, "all feature values should be commented out")
}

func TestGenerateExample_ContainsSections(t *testing.T) {
	output := GenerateExample(sampleDefaultKeys())

	assert.Contains(t, output, "[keys]")
	assert.Contains(t, output, "[keys.navigation]")
	assert.Contains(t, output, "[keys.search]")
	assert.Contains(t, output, "[keys.folds]")
	assert.Contains(t, output, "[keys.actions]")
	assert.Contains(t, output, "[keys.window]")
	assert.Contains(t, output, "[keys.visual]")
	assert.Contains(t, output, "[theme]")
	assert.Contains(t, output, "[theme.syntax]")
	assert.Contains(t, output, "[features]")
}

func TestGenerateExample_ContainsKeyDefaults(t *testing.T) {
	output := GenerateExample(sampleDefaultKeys())

	assert.Contains(t, output, `["up", "k"]`)
	assert.Contains(t, output, `["down", "j"]`)
	assert.Contains(t, output, `["q", "ctrl+c"]`)
	assert.Contains(t, output, `["g g"]`)
	assert.Contains(t, output, `["ctrl+w %"]`)
	assert.Contains(t, output, `["esc", "ctrl+g"]`)
	assert.Contains(t, output, `["space g j"]`)
	assert.Contains(t, output, `["space g k"]`)
}

func TestGenerateExample_ContainsThemeDefaults(t *testing.T) {
	output := GenerateExample(sampleDefaultKeys())

	assert.Contains(t, output, `"10"`)  // added
	assert.Contains(t, output, `"9"`)   // removed
	assert.Contains(t, output, `"12"`)  // changed
	assert.Contains(t, output, "green") // comment for added
}

func TestGenerateExample_ContainsFeatureDefaults(t *testing.T) {
	output := GenerateExample(sampleDefaultKeys())

	assert.Contains(t, output, "hscroll_step")
	assert.Contains(t, output, "= 4")
	assert.Contains(t, output, "commit_batch_size")
	assert.Contains(t, output, "= 100")
	assert.Contains(t, output, "snapshots")
	assert.Contains(t, output, "= true")
}

func TestGenerateExample_ContainsSyntaxDefaults(t *testing.T) {
	output := GenerateExample(sampleDefaultKeys())

	assert.Contains(t, output, "keyword")
	assert.Contains(t, output, "function")
	assert.Contains(t, output, "bright blue")    // function comment
	assert.Contains(t, output, "bright magenta") // type comment
}

func TestGenerateExample_ContainsSequenceNote(t *testing.T) {
	output := GenerateExample(sampleDefaultKeys())

	assert.Contains(t, output, "sequence")
}
