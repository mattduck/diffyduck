package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/diffyduck/pkg/config"
)

// KeyMap defines the key bindings for the application.
type KeyMap struct {
	// Navigation
	Up          []string
	Down        []string
	PageUp      []string
	PageDown    []string
	HalfUp      []string
	HalfDown    []string
	Top         []string
	Bottom      []string
	Left        []string
	Right       []string
	GoToTop     []string // sequence: "g g"
	NextHeading []string // sequence: "g j"
	PrevHeading []string // sequence: "g k"
	NextComment []string // sequence: "space c j"
	PrevComment []string // sequence: "space c k"
	NarrowNext  []string // next node while narrowed
	NarrowPrev  []string // previous node while narrowed

	// Search
	SearchForward []string
	SearchBack    []string
	NextMatch     []string
	PrevMatch     []string
	NarrowToggle  []string // toggle narrow mode: "space n f"

	// Folds
	FoldToggle     []string
	FoldToggleAll  []string
	FullFileToggle []string

	// Actions
	Quit           []string
	Enter          []string // used for comment mode
	Yank           []string
	YankAll        []string
	RefreshLayout  []string // recalculate dynamic column widths
	Snapshot       []string // take snapshot and switch to snapshot view
	SnapshotToggle []string // toggle snapshot view
	VisualMode     []string // enter visual line mode
	Help           []string // toggle help screen

	// Window management (sequences with prefix)
	WinSplitV      []string // "ctrl+w %"
	WinSplitH      []string // "ctrl+w \""
	WinClose       []string // "ctrl+w x"
	WinFocusLeft   []string // "ctrl+w h"
	WinFocusRight  []string // "ctrl+w l"
	WinFocusUp     []string // "ctrl+w k"
	WinFocusDown   []string // "ctrl+w j"
	WinResizeLeft  []string // "ctrl+w ctrl+h"
	WinResizeRight []string // "ctrl+w ctrl+l"
	WinResizeUp    []string // "ctrl+w ctrl+k"
	WinResizeDown  []string // "ctrl+w ctrl+j"

	// Visual mode
	VisualExit []string // exit visual mode: "esc", "ctrl+g"

	// prefixSet is built automatically from all bindings.
	// Contains all proper prefixes of multi-key sequences.
	// For "g j": {"g"}. For "space c j": {"space", "space c"}.
	prefixSet map[string]bool
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	km := KeyMap{
		Up:             []string{"up", "k"},
		Down:           []string{"down", "j"},
		PageUp:         []string{"pgup", "ctrl+b", "b"},
		PageDown:       []string{"pgdown", "ctrl+f", "f"},
		HalfUp:         []string{"ctrl+u", "u"},
		HalfDown:       []string{"ctrl+d", "d"},
		Top:            []string{"home"},
		Bottom:         []string{"end", "G"},
		Left:           []string{"left", "h"},
		Right:          []string{"right", "l"},
		GoToTop:        []string{"g g"},
		NextHeading:    []string{"g j"},
		PrevHeading:    []string{"g k"},
		NextComment:    []string{"space c j"},
		PrevComment:    []string{"space c k"},
		NarrowNext:     []string{"ctrl+j"},
		NarrowPrev:     []string{"ctrl+k"},
		Quit:           []string{"q", "ctrl+c"},
		SearchForward:  []string{"/"},
		SearchBack:     []string{"?"},
		NextMatch:      []string{"n"},
		PrevMatch:      []string{"N"},
		NarrowToggle:   []string{"space n f"},
		FoldToggle:     []string{"tab"},
		FoldToggleAll:  []string{"shift+tab"},
		FullFileToggle: []string{"F"},
		Enter:          []string{"C"},
		Yank:           []string{"y"},
		YankAll:        []string{"Y"},
		RefreshLayout:  []string{"r"},
		Snapshot:       []string{"S"},
		SnapshotToggle: []string{"s"},
		VisualMode:     []string{"V"},
		Help:           []string{"ctrl+h"},
		WinSplitV:      []string{"ctrl+w %"},
		WinSplitH:      []string{"ctrl+w \""},
		WinClose:       []string{"ctrl+w x"},
		WinFocusLeft:   []string{"ctrl+w h"},
		WinFocusRight:  []string{"ctrl+w l"},
		WinFocusUp:     []string{"ctrl+w k"},
		WinFocusDown:   []string{"ctrl+w j"},
		WinResizeLeft:  []string{"ctrl+w ctrl+h"},
		WinResizeRight: []string{"ctrl+w ctrl+l"},
		WinResizeUp:    []string{"ctrl+w ctrl+k"},
		WinResizeDown:  []string{"ctrl+w ctrl+j"},
		VisualExit:     []string{"esc", "ctrl+g"},
	}
	km.prefixSet = buildPrefixSet(km)
	return km
}

// Binding describes a single keybinding (or paired opposites) for help display.
// When Keys2/Desc2 are set, both halves render on one line separated by "/".
type Binding struct {
	Keys  []string // display keys, e.g. ["j", "↓"] or ["g g"] for sequences
	Desc  string   // human-readable description
	Keys2 []string // optional: paired opposite keys
	Desc2 string   // optional: paired opposite description
}

// BindingGroup is a named group of related bindings.
type BindingGroup struct {
	Name     string
	Bindings []Binding
}

// BindingGroups returns all binding groups for the help screen.
func (km KeyMap) BindingGroups() []BindingGroup {
	return []BindingGroup{
		{Name: "Navigation", Bindings: []Binding{
			{Keys: km.Down, Desc: "Scroll down", Keys2: km.Up, Desc2: "up"},
			{Keys: km.PageDown, Desc: "Page down", Keys2: km.PageUp, Desc2: "up"},
			{Keys: km.HalfDown, Desc: "Half page down", Keys2: km.HalfUp, Desc2: "up"},
			{Keys: km.Bottom, Desc: "Go to bottom", Keys2: km.Top, Desc2: "top"},
			{Keys: km.GoToTop, Desc: "Go to top"},
			{Keys: km.NextHeading, Desc: "Next heading", Keys2: km.PrevHeading, Desc2: "previous"},
			{Keys: km.NextComment, Desc: "Next comment", Keys2: km.PrevComment, Desc2: "previous"},
			{Keys: km.NarrowNext, Desc: "Narrow next", Keys2: km.NarrowPrev, Desc2: "previous"},
			{Keys: km.Right, Desc: "Scroll right", Keys2: km.Left, Desc2: "left"},
		}},
		{Name: "Search", Bindings: []Binding{
			{Keys: km.SearchForward, Desc: "Search forward", Keys2: km.SearchBack, Desc2: "backward"},
			{Keys: km.NextMatch, Desc: "Next match", Keys2: km.PrevMatch, Desc2: "previous"},
			{Keys: km.NarrowToggle, Desc: "Toggle narrow mode"},
			{Keys: []string{"Enter"}, Desc: "Execute search"},
			{Keys: []string{"Esc"}, Desc: "Cancel search"},
			{Keys: []string{"Backspace"}, Desc: "Delete character"},
		}},
		{Name: "Folds & Files", Bindings: []Binding{
			{Keys: km.FoldToggle, Desc: "Cycle fold level"},
			{Keys: km.FoldToggleAll, Desc: "Cycle fold level (all files)"},
			{Keys: km.FullFileToggle, Desc: "Toggle full file view"},
		}},
		{Name: "Actions", Bindings: []Binding{
			{Keys: km.Enter, Desc: "Add comment"},
			{Keys: km.Yank, Desc: "Copy item (SHA / path / comment)"},
			{Keys: km.YankAll, Desc: "Copy all comments"},
			{Keys: km.RefreshLayout, Desc: "Refresh layout"},
			{Keys: km.SnapshotToggle, Desc: "Toggle snapshot view"},
			{Keys: km.Snapshot, Desc: "Take snapshot"},
			{Keys: km.VisualMode, Desc: "Enter visual line mode"},
			{Keys: km.Help, Desc: "Toggle this help screen"},
			{Keys: km.Quit, Desc: "Quit"},
		}},
		{Name: "Window Management", Bindings: []Binding{
			{Keys: km.WinSplitV, Desc: "Vertical split"},
			{Keys: km.WinSplitH, Desc: "Horizontal split"},
			{Keys: km.WinClose, Desc: "Close window"},
			{Keys: km.WinFocusLeft, Desc: "Focus left", Keys2: km.WinFocusRight, Desc2: "right"},
			{Keys: km.WinFocusUp, Desc: "Focus up", Keys2: km.WinFocusDown, Desc2: "down"},
			{Keys: km.WinResizeLeft, Desc: "Resize left", Keys2: km.WinResizeRight, Desc2: "right"},
			{Keys: km.WinResizeUp, Desc: "Resize up", Keys2: km.WinResizeDown, Desc2: "down"},
		}},
		{Name: "Visual Mode", Bindings: []Binding{
			{Keys: km.VisualMode, Desc: "Enter visual line mode"},
			{Keys: km.Yank, Desc: "Yank selection"},
			{Keys: km.VisualExit, Desc: "Exit visual mode"},
		}},
		{Name: "Comment Editing", Bindings: []Binding{
			{Keys: km.Enter, Desc: "Start editing comment"},
			{Keys: []string{"C-j"}, Desc: "Submit comment"},
			{Keys: []string{"C-c", "C-g", "Esc"}, Desc: "Cancel comment"},
			{Keys: []string{"Enter"}, Desc: "Insert newline"},
			{Keys: []string{"C-a"}, Desc: "Start of line", Keys2: []string{"C-e"}, Desc2: "end"},
			{Keys: []string{"C-f"}, Desc: "Forward char", Keys2: []string{"C-b"}, Desc2: "backward"},
			{Keys: []string{"C-p"}, Desc: "Up line", Keys2: []string{"C-n"}, Desc2: "down"},
			{Keys: []string{"C-k"}, Desc: "Kill to end of line"},
			{Keys: []string{"C-u"}, Desc: "Kill to start of line"},
			{Keys: []string{"C-v"}, Desc: "Paste from clipboard"},
		}},
	}
}

// AllBindingGroups returns all binding groups for the help screen.
func AllBindingGroups(km KeyMap) []BindingGroup {
	return km.BindingGroups()
}

// parseBinding splits a binding string on spaces.
// Single keys return a 1-element slice; sequences like "g g" return ["g", "g"].
func parseBinding(s string) []string {
	return strings.Fields(s)
}

// keyToken normalizes a tea.KeyMsg to its canonical token string.
// The literal space key " " maps to "space" so it can appear in
// space-separated binding strings like "space c j".
func keyToken(msg tea.KeyMsg) string {
	if msg.String() == " " {
		return "space"
	}
	return msg.String()
}

// buildPrefixSet scans all bindings and collects ALL proper prefixes of
// multi-key sequences. For "g j": adds "g". For "space c j": adds
// "space" and "space c".
func buildPrefixSet(km KeyMap) map[string]bool {
	set := make(map[string]bool)
	for _, keys := range allBindings(km) {
		for _, k := range keys {
			parts := parseBinding(k)
			// Add every proper prefix (all but the full sequence)
			for i := 1; i < len(parts); i++ {
				set[strings.Join(parts[:i], " ")] = true
			}
		}
	}
	return set
}

// matchesSequence checks if prefix + " " + keyToken(msg) matches any binding in keys.
func matchesSequence(prefix string, msg tea.KeyMsg, keys []string) bool {
	seq := prefix + " " + keyToken(msg)
	for _, k := range keys {
		if k == seq {
			return true
		}
	}
	return false
}

// matchesKey checks if a key message matches any of the given key strings.
// Sequence bindings (multi-token strings like "g g") are skipped —
// use matchesSequence for those. The literal space key " " is not a sequence.
func matchesKey(msg tea.KeyMsg, keys []string) bool {
	s := msg.String()
	for _, k := range keys {
		if len(parseBinding(k)) > 1 {
			continue // skip sequence bindings
		}
		if s == k {
			return true
		}
	}
	return false
}

// ValidateBindings checks for conflicts where a key is both a single-key
// binding and a sequence prefix. Returns an error describing the first conflict.
func ValidateBindings(km KeyMap) error {
	singleKeys := make(map[string]bool)
	for _, keys := range allBindings(km) {
		for _, k := range keys {
			if len(parseBinding(k)) <= 1 {
				// Normalize " " → "space" to match prefix set convention
				token := k
				if token == " " {
					token = "space"
				}
				singleKeys[token] = true
			}
		}
	}
	for prefix := range km.prefixSet {
		if singleKeys[prefix] {
			return fmt.Errorf("key %q is both a direct binding and a sequence prefix — the sequence will never fire", prefix)
		}
	}
	return nil
}

// allBindings returns all binding slices from the KeyMap.
func allBindings(km KeyMap) [][]string {
	return [][]string{
		km.Up, km.Down, km.PageUp, km.PageDown,
		km.HalfUp, km.HalfDown, km.Top, km.Bottom,
		km.Left, km.Right, km.GoToTop, km.NextHeading, km.PrevHeading,
		km.NextComment, km.PrevComment,
		km.NarrowNext, km.NarrowPrev,
		km.SearchForward, km.SearchBack, km.NextMatch, km.PrevMatch,
		km.NarrowToggle,
		km.FoldToggle, km.FoldToggleAll, km.FullFileToggle,
		km.Quit, km.Enter, km.Yank, km.YankAll,
		km.RefreshLayout, km.Snapshot, km.SnapshotToggle, km.VisualMode, km.Help,
		km.WinSplitV, km.WinSplitH, km.WinClose,
		km.WinFocusLeft, km.WinFocusRight, km.WinFocusUp, km.WinFocusDown,
		km.WinResizeLeft, km.WinResizeRight, km.WinResizeUp, km.WinResizeDown,
		km.VisualExit,
	}
}

// ApplyKeysConfig returns a KeyMap with defaults overridden by any non-nil
// slices in the config. Actions not mentioned in config keep their defaults.
// Nil subsection pointers mean "use all defaults for that section".
func ApplyKeysConfig(cfg config.KeysConfig) KeyMap {
	km := DefaultKeyMap()

	if nav := cfg.Navigation; nav != nil {
		if nav.Up != nil {
			km.Up = nav.Up
		}
		if nav.Down != nil {
			km.Down = nav.Down
		}
		if nav.PageUp != nil {
			km.PageUp = nav.PageUp
		}
		if nav.PageDown != nil {
			km.PageDown = nav.PageDown
		}
		if nav.HalfUp != nil {
			km.HalfUp = nav.HalfUp
		}
		if nav.HalfDown != nil {
			km.HalfDown = nav.HalfDown
		}
		if nav.Top != nil {
			km.Top = nav.Top
		}
		if nav.Bottom != nil {
			km.Bottom = nav.Bottom
		}
		if nav.Left != nil {
			km.Left = nav.Left
		}
		if nav.Right != nil {
			km.Right = nav.Right
		}
		if nav.GoToTop != nil {
			km.GoToTop = nav.GoToTop
		}
		if nav.NextHeading != nil {
			km.NextHeading = nav.NextHeading
		}
		if nav.PrevHeading != nil {
			km.PrevHeading = nav.PrevHeading
		}
		if nav.NextComment != nil {
			km.NextComment = nav.NextComment
		}
		if nav.PrevComment != nil {
			km.PrevComment = nav.PrevComment
		}
		if nav.NarrowNext != nil {
			km.NarrowNext = nav.NarrowNext
		}
		if nav.NarrowPrev != nil {
			km.NarrowPrev = nav.NarrowPrev
		}
	}

	if s := cfg.Search; s != nil {
		if s.SearchFwd != nil {
			km.SearchForward = s.SearchFwd
		}
		if s.SearchBack != nil {
			km.SearchBack = s.SearchBack
		}
		if s.NextMatch != nil {
			km.NextMatch = s.NextMatch
		}
		if s.PrevMatch != nil {
			km.PrevMatch = s.PrevMatch
		}
		if s.NarrowToggle != nil {
			km.NarrowToggle = s.NarrowToggle
		}
	}

	if f := cfg.Folds; f != nil {
		if f.Fold != nil {
			km.FoldToggle = f.Fold
		}
		if f.FoldAll != nil {
			km.FoldToggleAll = f.FoldAll
		}
		if f.FullFile != nil {
			km.FullFileToggle = f.FullFile
		}
	}

	if a := cfg.Actions; a != nil {
		if a.Quit != nil {
			km.Quit = a.Quit
		}
		if a.Enter != nil {
			km.Enter = a.Enter
		}
		if a.Yank != nil {
			km.Yank = a.Yank
		}
		if a.YankAll != nil {
			km.YankAll = a.YankAll
		}
		if a.Refresh != nil {
			km.RefreshLayout = a.Refresh
		}
		if a.Snapshot != nil {
			km.Snapshot = a.Snapshot
		}
		if a.SnapshotToggle != nil {
			km.SnapshotToggle = a.SnapshotToggle
		}
		if a.Visual != nil {
			km.VisualMode = a.Visual
		}
		if a.Help != nil {
			km.Help = a.Help
		}
	}

	if w := cfg.Window; w != nil {
		if w.SplitVertical != nil {
			km.WinSplitV = w.SplitVertical
		}
		if w.SplitHorizontal != nil {
			km.WinSplitH = w.SplitHorizontal
		}
		if w.Close != nil {
			km.WinClose = w.Close
		}
		if w.FocusLeft != nil {
			km.WinFocusLeft = w.FocusLeft
		}
		if w.FocusRight != nil {
			km.WinFocusRight = w.FocusRight
		}
		if w.FocusUp != nil {
			km.WinFocusUp = w.FocusUp
		}
		if w.FocusDown != nil {
			km.WinFocusDown = w.FocusDown
		}
		if w.ResizeLeft != nil {
			km.WinResizeLeft = w.ResizeLeft
		}
		if w.ResizeRight != nil {
			km.WinResizeRight = w.ResizeRight
		}
		if w.ResizeUp != nil {
			km.WinResizeUp = w.ResizeUp
		}
		if w.ResizeDown != nil {
			km.WinResizeDown = w.ResizeDown
		}
	}

	if v := cfg.Visual; v != nil {
		if v.Exit != nil {
			km.VisualExit = v.Exit
		}
	}

	// Rebuild prefix set after applying overrides
	km.prefixSet = buildPrefixSet(km)
	return km
}

// DefaultKeysConfig returns the default key bindings as a config.KeysConfig,
// for use by GenerateExample.
func DefaultKeysConfig() config.KeysConfig {
	km := DefaultKeyMap()
	return config.KeysConfig{
		Navigation: &config.NavigationKeys{
			Up:          km.Up,
			Down:        km.Down,
			PageUp:      km.PageUp,
			PageDown:    km.PageDown,
			HalfUp:      km.HalfUp,
			HalfDown:    km.HalfDown,
			Top:         km.Top,
			Bottom:      km.Bottom,
			Left:        km.Left,
			Right:       km.Right,
			GoToTop:     km.GoToTop,
			NextHeading: km.NextHeading,
			PrevHeading: km.PrevHeading,
			NextComment: km.NextComment,
			PrevComment: km.PrevComment,
			NarrowNext:  km.NarrowNext,
			NarrowPrev:  km.NarrowPrev,
		},
		Search: &config.SearchKeys{
			SearchFwd:    km.SearchForward,
			SearchBack:   km.SearchBack,
			NextMatch:    km.NextMatch,
			PrevMatch:    km.PrevMatch,
			NarrowToggle: km.NarrowToggle,
		},
		Folds: &config.FoldKeys{
			Fold:     km.FoldToggle,
			FoldAll:  km.FoldToggleAll,
			FullFile: km.FullFileToggle,
		},
		Actions: &config.ActionKeys{
			Quit:           km.Quit,
			Enter:          km.Enter,
			Yank:           km.Yank,
			YankAll:        km.YankAll,
			Refresh:        km.RefreshLayout,
			Snapshot:       km.Snapshot,
			SnapshotToggle: km.SnapshotToggle,
			Visual:         km.VisualMode,
			Help:           km.Help,
		},
		Window: &config.WindowKeys{
			SplitVertical:   km.WinSplitV,
			SplitHorizontal: km.WinSplitH,
			Close:           km.WinClose,
			FocusLeft:       km.WinFocusLeft,
			FocusRight:      km.WinFocusRight,
			FocusUp:         km.WinFocusUp,
			FocusDown:       km.WinFocusDown,
			ResizeLeft:      km.WinResizeLeft,
			ResizeRight:     km.WinResizeRight,
			ResizeUp:        km.WinResizeUp,
			ResizeDown:      km.WinResizeDown,
		},
		Visual: &config.VisualKeys{
			Exit: km.VisualExit,
		},
	}
}
