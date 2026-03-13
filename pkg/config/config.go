package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config is the top-level user configuration.
type Config struct {
	Keys     KeysConfig     `toml:"keys"`
	Theme    ThemeConfig    `toml:"theme"`
	Features FeaturesConfig `toml:"features"`
}

// KeysConfig groups key bindings by mode/purpose.
// Nil pointer = use all defaults for that section.
type KeysConfig struct {
	Navigation *NavigationKeys `toml:"navigation"`
	Search     *SearchKeys     `toml:"search"`
	Folds      *FoldKeys       `toml:"folds"`
	Actions    *ActionKeys     `toml:"actions"`
	Window     *WindowKeys     `toml:"window"`
	Visual     *VisualKeys     `toml:"visual"`
}

// NavigationKeys configures movement bindings.
// Key sequences use space-separated strings: "g g", "space c j".
// The literal space key uses the token "space" in sequences.
type NavigationKeys struct {
	Up             []string `toml:"up"`
	Down           []string `toml:"down"`
	PageUp         []string `toml:"page_up"`
	PageDown       []string `toml:"page_down"`
	HalfUp         []string `toml:"half_up"`
	HalfDown       []string `toml:"half_down"`
	Top            []string `toml:"top"`
	Bottom         []string `toml:"bottom"`
	Left           []string `toml:"left"`
	Right          []string `toml:"right"`
	GoToTop        []string `toml:"go_to_top"`        // default: ["g g"]
	NextHeading    []string `toml:"next_heading"`     // default: ["g j"]
	PrevHeading    []string `toml:"prev_heading"`     // default: ["g k"]
	NextComment    []string `toml:"next_comment"`     // default: ["space c j"]
	PrevComment    []string `toml:"prev_comment"`     // default: ["space c k"]
	NextAllComment []string `toml:"next_all_comment"` // default: ["space C j"]
	PrevAllComment []string `toml:"prev_all_comment"` // default: ["space C k"]
	NextChange     []string `toml:"next_change"`      // default: ["space g j"]
	PrevChange     []string `toml:"prev_change"`      // default: ["space g k"]
	NarrowNext     []string `toml:"narrow_next"`      // default: ["ctrl+j"]
	NarrowPrev     []string `toml:"narrow_prev"`      // default: ["ctrl+k"]
}

// SearchKeys configures search bindings.
type SearchKeys struct {
	SearchFwd    []string `toml:"search_fwd"`
	SearchBack   []string `toml:"search_back"`
	NextMatch    []string `toml:"next_match"`
	PrevMatch    []string `toml:"prev_match"`
	NarrowToggle []string `toml:"narrow_toggle"` // default: ["space n f"]
}

// FoldKeys configures fold/expand bindings.
type FoldKeys struct {
	Fold     []string `toml:"fold"`
	FoldAll  []string `toml:"fold_all"`
	FullFile []string `toml:"full_file"`
}

// ActionKeys configures action bindings.
type ActionKeys struct {
	Quit            []string `toml:"quit"`
	Enter           []string `toml:"enter"`
	ResolveToggle   []string `toml:"resolve_toggle"`
	Yank            []string `toml:"yank"`
	YankUnresolved  []string `toml:"yank_unresolved"`   // default: ["space c y"]
	YankAllComments []string `toml:"yank_all_comments"` // default: ["space C y"]
	Refresh         []string `toml:"refresh"`
	Snapshot        []string `toml:"snapshot"`
	SnapshotToggle  []string `toml:"snapshot_toggle"`
	Visual          []string `toml:"visual"`
	Help            []string `toml:"help"`           // default: ["ctrl+h"]
	MoveDetect      []string `toml:"move_detect"`    // default: ["M"]
	CommentToggle   []string `toml:"comment_toggle"` // default: ["C"]
}

// WindowKeys configures window management bindings.
// All defaults are key sequences with "ctrl+w" prefix.
type WindowKeys struct {
	SplitVertical   []string `toml:"split_vertical"`   // default: ["ctrl+w %"]
	SplitHorizontal []string `toml:"split_horizontal"` // default: ["ctrl+w \""]
	Close           []string `toml:"close"`            // default: ["ctrl+w x"]
	FocusLeft       []string `toml:"focus_left"`       // default: ["ctrl+w h"]
	FocusRight      []string `toml:"focus_right"`      // default: ["ctrl+w l"]
	FocusUp         []string `toml:"focus_up"`         // default: ["ctrl+w k"]
	FocusDown       []string `toml:"focus_down"`       // default: ["ctrl+w j"]
	ResizeLeft      []string `toml:"resize_left"`      // default: ["ctrl+w ctrl+h"]
	ResizeRight     []string `toml:"resize_right"`     // default: ["ctrl+w ctrl+l"]
	ResizeUp        []string `toml:"resize_up"`        // default: ["ctrl+w ctrl+k"]
	ResizeDown      []string `toml:"resize_down"`      // default: ["ctrl+w ctrl+j"]
}

// VisualKeys configures visual line mode bindings.
type VisualKeys struct {
	Exit []string `toml:"exit"` // default: ["esc", "ctrl+g"]
}

// ThemeConfig holds color values as strings.
// Empty string means "use default". Values are ANSI 256-color numbers ("0"-"255")
// or hex strings ("#ff5733").
type ThemeConfig struct {
	Added           string `toml:"added"`
	Removed         string `toml:"removed"`
	Changed         string `toml:"changed"`
	Context         string `toml:"context"`
	LineNumber      string `toml:"line_number"`
	Header          string `toml:"header"`
	HeaderDir       string `toml:"header_dir"`
	HeaderLine      string `toml:"header_line"`
	HunkSeparator   string `toml:"hunk_separator"`
	SearchMatchFg   string `toml:"search_match_fg"`
	SearchMatchBg   string `toml:"search_match_bg"`
	SearchCurrentFg string `toml:"search_current_fg"`
	SearchCurrentBg string `toml:"search_current_bg"`
	CursorBg        string `toml:"cursor_bg"`
	CursorFg        string `toml:"cursor_fg"`
	CursorArrow     string `toml:"cursor_arrow"`
	StatusBg        string `toml:"status_bg"`
	StatusFg        string `toml:"status_fg"`
	CommitTree      string `toml:"commit_tree"`
	SnapshotTree    string `toml:"snapshot_tree"`
	LocalRef        string `toml:"local_ref"`
	RemoteRef       string `toml:"remote_ref"`
	ConflictMarker  string `toml:"conflict_marker"`
	CommentCheckbox string `toml:"comment_checkbox"`

	Syntax *SyntaxConfig `toml:"syntax"`
}

// SyntaxConfig holds syntax highlighting colors for code.
// Nil pointer = use all defaults. Empty string = use default for that category.
// Values are ANSI 256-color numbers ("0"-"255") or hex strings ("#ff5733").
type SyntaxConfig struct {
	Keyword   string `toml:"keyword"`   // also: control flow, storage class
	String    string `toml:"string"`    // string literals
	Number    string `toml:"number"`    // also: boolean, nil, constant
	Comment   string `toml:"comment"`   // also: doc comments
	Function  string `toml:"function"`  // definitions and call sites
	Type      string `toml:"type"`      // type names
	Field     string `toml:"field"`     // struct/object fields
	Operator  string `toml:"operator"`  // also: punctuation
	Tag       string `toml:"tag"`       // also: attributes/decorators
	Namespace string `toml:"namespace"` // package/module names
	Variable  string `toml:"variable"`  // also: parameters
}

// FeaturesConfig holds behavioral settings.
// Pointer types distinguish "not set" from zero-value.
type FeaturesConfig struct {
	HScrollStep     *int  `toml:"hscroll_step"`
	CommitBatchSize *int  `toml:"commit_batch_size"`
	AutoSnapshots   *bool `toml:"auto_snapshots"`    // take snapshots automatically (default: true)
	ShowSnapshots   *bool `toml:"show_snapshots"`    // show snapshot view by default (default: false)
	ExpandAllBudget *int  `toml:"expand_all_budget"` // max files for full shift-tab expansion (default: 500)
	ChordTimeoutMs  *int  `toml:"chord_timeout_ms"`  // dual-use prefix timeout in milliseconds (default: 250)
	AutoUnfoldLimit *int  `toml:"auto_unfold_limit"` // max rows to auto-unfold on startup (default: 800)
	MoveDetectMin   *int  `toml:"move_detect_min"`   // min consecutive lines for move detection (default: 2)
}

// Load reads the config file from the XDG-conventional path.
// Returns a zero Config (all defaults) if no file exists.
// Returns an error only for malformed TOML or I/O errors other than missing file.
func Load() (Config, error) {
	return LoadFrom(Path())
}

// LoadFrom reads a config from the given file path.
// Returns a zero Config if the file does not exist.
func LoadFrom(path string) (Config, error) {
	var cfg Config
	_, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		if hasOldFlatKeys(path) {
			return Config{}, fmt.Errorf(
				"config format has changed: [keys] now uses subsections like "+
					"[keys.navigation], [keys.actions], etc. "+
					"Run 'dfd config --print' to see the new format: %w", err)
		}
		return Config{}, err
	}
	// Check for old flat [keys] format even after successful decode,
	// because TOML silently ignores unknown keys under [keys].
	if hasOldFlatKeys(path) {
		return Config{}, fmt.Errorf(
			"config format has changed: [keys] now uses subsections like " +
				"[keys.navigation], [keys.actions], etc. " +
				"Run 'dfd config --print' to see the new format")
	}
	return cfg, nil
}

// Path returns the resolved config file path, respecting XDG_CONFIG_HOME.
func Path() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "dfd", "config.toml")
}

// hasOldFlatKeys checks whether a config file uses the old flat [keys] format
// (e.g. up = [...] directly under [keys] instead of [keys.navigation]).
func hasOldFlatKeys(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	// Old flat keys that would appear directly under [keys]
	oldKeys := []string{"up =", "down =", "quit =", "page_up =", "search_fwd =", "fold ="}

	scanner := bufio.NewScanner(f)
	inKeysSection := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "[keys]" {
			inKeysSection = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inKeysSection = false
			continue
		}
		if inKeysSection {
			for _, old := range oldKeys {
				if strings.HasPrefix(line, old) {
					return true
				}
			}
		}
	}
	return false
}
