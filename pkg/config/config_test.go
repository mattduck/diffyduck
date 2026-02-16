package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_MissingFile(t *testing.T) {
	cfg, err := LoadFrom("/nonexistent/path/config.toml")
	require.NoError(t, err)
	assert.Equal(t, Config{}, cfg)
}

func TestLoad_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(""), 0o644))

	cfg, err := LoadFrom(path)
	require.NoError(t, err)
	assert.Equal(t, Config{}, cfg)
}

func TestLoad_PartialConfig_NavigationOnly(t *testing.T) {
	content := `
[keys.navigation]
up = ["w"]
`
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, err := LoadFrom(path)
	require.NoError(t, err)

	require.NotNil(t, cfg.Keys.Navigation)
	assert.Equal(t, []string{"w"}, cfg.Keys.Navigation.Up)
	// Unmentioned keys remain nil (zero value)
	assert.Nil(t, cfg.Keys.Navigation.Down)
	assert.Nil(t, cfg.Keys.Navigation.PageUp)
	// Other subsections not set
	assert.Nil(t, cfg.Keys.Search)
	assert.Nil(t, cfg.Keys.Actions)
}

func TestLoad_PartialConfig_ActionsOnly(t *testing.T) {
	content := `
[keys.actions]
quit = ["q", "Z", "Q"]
help = ["ctrl+h", "?"]
`
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, err := LoadFrom(path)
	require.NoError(t, err)

	require.NotNil(t, cfg.Keys.Actions)
	assert.Equal(t, []string{"q", "Z", "Q"}, cfg.Keys.Actions.Quit)
	assert.Equal(t, []string{"ctrl+h", "?"}, cfg.Keys.Actions.Help)
	assert.Nil(t, cfg.Keys.Actions.Enter)
}

func TestLoad_PartialConfig_ThemeOnly(t *testing.T) {
	content := `
[theme]
added = "#00ff00"
removed = "196"
`
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, err := LoadFrom(path)
	require.NoError(t, err)

	assert.Equal(t, "#00ff00", cfg.Theme.Added)
	assert.Equal(t, "196", cfg.Theme.Removed)
	assert.Equal(t, "", cfg.Theme.Changed) // unmentioned
}

func TestLoad_PartialConfig_FeaturesOnly(t *testing.T) {
	content := `
[features]
hscroll_step = 8
auto_snapshots = false
show_snapshots = true
`
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, err := LoadFrom(path)
	require.NoError(t, err)

	require.NotNil(t, cfg.Features.HScrollStep)
	assert.Equal(t, 8, *cfg.Features.HScrollStep)
	require.NotNil(t, cfg.Features.AutoSnapshots)
	assert.Equal(t, false, *cfg.Features.AutoSnapshots)
	require.NotNil(t, cfg.Features.ShowSnapshots)
	assert.Equal(t, true, *cfg.Features.ShowSnapshots)
	assert.Nil(t, cfg.Features.CommitBatchSize) // unmentioned
	assert.Nil(t, cfg.Features.ExpandAllBudget) // unmentioned
}

func TestLoad_ExpandAllBudget(t *testing.T) {
	content := `
[features]
expand_all_budget = 200
`
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, err := LoadFrom(path)
	require.NoError(t, err)

	require.NotNil(t, cfg.Features.ExpandAllBudget)
	assert.Equal(t, 200, *cfg.Features.ExpandAllBudget)
}

func TestLoad_SyntaxConfig(t *testing.T) {
	content := `
[theme.syntax]
keyword = "#ff0000"
function = "12"
comment = "8"
`
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, err := LoadFrom(path)
	require.NoError(t, err)

	require.NotNil(t, cfg.Theme.Syntax)
	assert.Equal(t, "#ff0000", cfg.Theme.Syntax.Keyword)
	assert.Equal(t, "12", cfg.Theme.Syntax.Function)
	assert.Equal(t, "8", cfg.Theme.Syntax.Comment)
	assert.Equal(t, "", cfg.Theme.Syntax.String)   // unmentioned
	assert.Equal(t, "", cfg.Theme.Syntax.Variable) // unmentioned
	assert.Nil(t, cfg.Keys.Navigation)             // no keys at all
}

func TestLoad_SyntaxConfig_Nil(t *testing.T) {
	content := `
[theme]
added = "10"
`
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, err := LoadFrom(path)
	require.NoError(t, err)

	assert.Nil(t, cfg.Theme.Syntax, "syntax should be nil when [theme.syntax] is absent")
	assert.Equal(t, "10", cfg.Theme.Added)
}

func TestLoad_FullConfig(t *testing.T) {
	content := `
[keys.navigation]
up = ["up", "k"]
down = ["down", "j"]
page_up = ["pgup"]
page_down = ["pgdown"]
half_up = ["ctrl+u"]
half_down = ["ctrl+d"]
top = ["home"]
bottom = ["end"]
left = ["left"]
right = ["right"]
go_to_top = ["g g"]
next_heading = ["g j"]
prev_heading = ["g k"]
next_comment = ["space c j"]
prev_comment = ["space c k"]
next_change = ["space g j"]
prev_change = ["space g k"]
narrow_next = ["ctrl+j"]
narrow_prev = ["ctrl+k"]

[keys.search]
search_fwd = ["/"]
search_back = ["?"]
next_match = ["n"]
prev_match = ["N"]

[keys.folds]
fold = ["tab"]
fold_all = ["shift+tab"]
full_file = ["F"]

[keys.actions]
quit = ["q"]
enter = ["enter"]
yank = ["y"]
yank_all = ["Y"]
refresh = ["r"]
snapshot = ["R"]
visual = ["V"]
help = ["ctrl+h"]

[keys.window]
split_vertical = ["ctrl+w %"]
split_horizontal = ["ctrl+w \""]
close = ["ctrl+w x"]
focus_left = ["ctrl+w h"]
focus_right = ["ctrl+w l"]
focus_up = ["ctrl+w k"]
focus_down = ["ctrl+w j"]
resize_left = ["ctrl+w ctrl+h"]
resize_right = ["ctrl+w ctrl+l"]
resize_up = ["ctrl+w ctrl+k"]
resize_down = ["ctrl+w ctrl+j"]

[keys.visual]
exit = ["esc", "ctrl+g"]

[theme]
added = "10"
removed = "9"
changed = "12"
context = ""
line_number = "8"
header = "15"
header_dir = "7"
header_line = "8"
hunk_separator = "8"
search_match_fg = "0"
search_match_bg = "3"
search_current_fg = "0"
search_current_bg = "9"
cursor_bg = "7"
cursor_fg = "0"
cursor_arrow = "15"
status_bg = "8"
status_fg = "0"
commit_tree = "3"
snapshot_tree = "5"
local_ref = "6"
remote_ref = "8"
conflict_marker = "3"

[features]
hscroll_step = 4
commit_batch_size = 100
auto_snapshots = true
show_snapshots = false
`
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, err := LoadFrom(path)
	require.NoError(t, err)

	// Spot-check across all sections
	require.NotNil(t, cfg.Keys.Navigation)
	assert.Equal(t, []string{"up", "k"}, cfg.Keys.Navigation.Up)
	assert.Equal(t, []string{"g g"}, cfg.Keys.Navigation.GoToTop)
	assert.Equal(t, []string{"space g j"}, cfg.Keys.Navigation.NextChange)
	assert.Equal(t, []string{"space g k"}, cfg.Keys.Navigation.PrevChange)
	require.NotNil(t, cfg.Keys.Actions)
	assert.Equal(t, []string{"V"}, cfg.Keys.Actions.Visual)
	assert.Equal(t, []string{"ctrl+h"}, cfg.Keys.Actions.Help)
	require.NotNil(t, cfg.Keys.Window)
	assert.Equal(t, []string{"ctrl+w %"}, cfg.Keys.Window.SplitVertical)
	require.NotNil(t, cfg.Keys.Visual)
	assert.Equal(t, []string{"esc", "ctrl+g"}, cfg.Keys.Visual.Exit)
	assert.Equal(t, "10", cfg.Theme.Added)
	assert.Equal(t, "3", cfg.Theme.ConflictMarker)
	require.NotNil(t, cfg.Features.HScrollStep)
	assert.Equal(t, 4, *cfg.Features.HScrollStep)
	require.NotNil(t, cfg.Features.AutoSnapshots)
	assert.Equal(t, true, *cfg.Features.AutoSnapshots)
	require.NotNil(t, cfg.Features.ShowSnapshots)
	assert.Equal(t, false, *cfg.Features.ShowSnapshots)
}

func TestLoad_MalformedTOML(t *testing.T) {
	content := `[keys
this is not valid toml!!!
`
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	_, err := LoadFrom(path)
	assert.Error(t, err)
}

func TestLoad_UnknownKeysIgnored(t *testing.T) {
	content := `
[keys.navigation]
up = ["w"]
nonexistent_action = ["x"]

[theme]
unknown_color = "99"

[totally_unknown_section]
foo = "bar"
`
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, err := LoadFrom(path)
	require.NoError(t, err)
	require.NotNil(t, cfg.Keys.Navigation)
	assert.Equal(t, []string{"w"}, cfg.Keys.Navigation.Up)
}

func TestLoad_OldFlatFormat(t *testing.T) {
	content := `
[keys]
up = ["w"]
down = ["s"]
`
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	_, err := LoadFrom(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config format has changed")
	assert.Contains(t, err.Error(), "subsections")
}

func TestLoad_SequenceBindings(t *testing.T) {
	content := `
[keys.navigation]
go_to_top = ["z z"]
next_heading = ["z j"]
prev_heading = ["z k"]
`
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, err := LoadFrom(path)
	require.NoError(t, err)
	require.NotNil(t, cfg.Keys.Navigation)
	assert.Equal(t, []string{"z z"}, cfg.Keys.Navigation.GoToTop)
	assert.Equal(t, []string{"z j"}, cfg.Keys.Navigation.NextHeading)
	assert.Equal(t, []string{"z k"}, cfg.Keys.Navigation.PrevHeading)
}

func TestLoad_PartialSubsection(t *testing.T) {
	// Only one subsection set, others nil
	content := `
[keys.folds]
fold = ["z"]
`
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, err := LoadFrom(path)
	require.NoError(t, err)
	assert.Nil(t, cfg.Keys.Navigation)
	assert.Nil(t, cfg.Keys.Search)
	require.NotNil(t, cfg.Keys.Folds)
	assert.Equal(t, []string{"z"}, cfg.Keys.Folds.Fold)
	assert.Nil(t, cfg.Keys.Folds.FoldAll) // unmentioned within subsection
	assert.Nil(t, cfg.Keys.Actions)
	assert.Nil(t, cfg.Keys.Window)
	assert.Nil(t, cfg.Keys.Visual)
}

func TestPath_XDGConfigHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	got := Path()
	assert.Equal(t, filepath.Join(dir, "dfd", "config.toml"), got)
}

func TestPath_DefaultFallback(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	got := Path()
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".config", "dfd", "config.toml"), got)
}

func TestLoad_ViaXDGPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	configDir := filepath.Join(dir, "dfd")
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	content := `
[keys.navigation]
up = ["w"]
`
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(content), 0o644))

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg.Keys.Navigation)
	assert.Equal(t, []string{"w"}, cfg.Keys.Navigation.Up)
}

func TestLoad_NoFileAtXDGPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, Config{}, cfg)
}
