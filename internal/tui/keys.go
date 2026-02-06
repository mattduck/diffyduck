package tui

import "github.com/charmbracelet/bubbletea"

// KeyMap defines the key bindings for the application.
type KeyMap struct {
	Up             []string
	Down           []string
	PageUp         []string
	PageDown       []string
	HalfUp         []string
	HalfDown       []string
	Top            []string
	Bottom         []string
	Left           []string
	Right          []string
	Quit           []string
	SearchForward  []string
	SearchBack     []string
	NextMatch      []string
	PrevMatch      []string
	NarrowToggle   []string // toggle narrow mode (shares key with PrevMatch when no search active)
	FoldToggle     []string
	FoldToggleAll  []string
	FullFileToggle []string
	Enter          []string // used for comment mode
	Yank           []string
	YankAll        []string
	RefreshLayout  []string // recalculate dynamic column widths
	Snapshot       []string // create snapshot and show incremental diff
	VisualMode     []string // enter visual line mode
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:             []string{"up", "k"},
		Down:           []string{"down", "j"},
		PageUp:         []string{"pgup", "ctrl+b", "b"},
		PageDown:       []string{"pgdown", "ctrl+f", " ", "f"},
		HalfUp:         []string{"ctrl+u", "u"},
		HalfDown:       []string{"ctrl+d", "d"},
		Top:            []string{"home"},
		Bottom:         []string{"end", "G"},
		Left:           []string{"left", "h"},
		Right:          []string{"right", "l"},
		Quit:           []string{"q", "ctrl+c"},
		SearchForward:  []string{"/"},
		SearchBack:     []string{"?"},
		NextMatch:      []string{"n"},
		PrevMatch:      []string{"N"},
		NarrowToggle:   []string{"N"}, // same key as PrevMatch; active when no search query
		FoldToggle:     []string{"tab"},
		FoldToggleAll:  []string{"shift+tab"},
		FullFileToggle: []string{"F"},
		Enter:          []string{"enter"},
		Yank:           []string{"y"},
		YankAll:        []string{"Y"},
		RefreshLayout:  []string{"r"},
		Snapshot:       []string{"R"},
		VisualMode:     []string{"V"},
	}
}

// Binding describes a single keybinding (or paired opposites) for help display.
// When Keys2/Desc2 are set, both halves render on one line separated by "/".
type Binding struct {
	Keys  []string // display keys, e.g. ["j", "↓"] or ["gg"] for sequences
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
// g-sequences are folded into their relevant sections, search input is
// merged with search, and mode-entry keys are duplicated in the mode section.
func (km KeyMap) BindingGroups() []BindingGroup {
	return []BindingGroup{
		{Name: "Navigation", Bindings: []Binding{
			{Keys: km.Down, Desc: "Scroll down", Keys2: km.Up, Desc2: "up"},
			{Keys: km.PageDown, Desc: "Page down", Keys2: km.PageUp, Desc2: "up"},
			{Keys: km.HalfDown, Desc: "Half page down", Keys2: km.HalfUp, Desc2: "up"},
			{Keys: km.Bottom, Desc: "Go to bottom", Keys2: km.Top, Desc2: "top"},
			{Keys: []string{"gg"}, Desc: "Go to top"},
			{Keys: []string{"gj"}, Desc: "Next heading", Keys2: []string{"gk"}, Desc2: "previous"},
			{Keys: km.Right, Desc: "Scroll right", Keys2: km.Left, Desc2: "left"},
		}},
		{Name: "Search", Bindings: []Binding{
			{Keys: km.SearchForward, Desc: "Search forward", Keys2: km.SearchBack, Desc2: "backward"},
			{Keys: km.NextMatch, Desc: "Next match", Keys2: km.PrevMatch, Desc2: "previous"},
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
			{Keys: km.Yank, Desc: "Copy current line"},
			{Keys: km.YankAll, Desc: "Copy all visible content"},
			{Keys: km.RefreshLayout, Desc: "Refresh layout"},
			{Keys: km.Snapshot, Desc: "Create snapshot"},
			{Keys: km.VisualMode, Desc: "Enter visual line mode"},
			{Keys: []string{"ctrl+h"}, Desc: "Toggle this help screen"},
			{Keys: km.Quit, Desc: "Quit"},
		}},
		{Name: "Window Management", Bindings: []Binding{
			{Keys: []string{"C-w %"}, Desc: "Vertical split"},
			{Keys: []string{"C-w \""}, Desc: "Horizontal split"},
			{Keys: []string{"C-w x"}, Desc: "Close window"},
			{Keys: []string{"C-w h/j/k/l"}, Desc: "Focus left/down/up/right"},
			{Keys: []string{"C-w C-h/C-j/C-k/C-l"}, Desc: "Resize left/down/up/right"},
		}},
		{Name: "Visual Mode", Bindings: []Binding{
			{Keys: km.VisualMode, Desc: "Enter visual line mode"},
			{Keys: []string{"Esc", "C-g"}, Desc: "Exit visual mode"},
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

// matchesKey checks if a key message matches any of the given key strings.
func matchesKey(msg tea.KeyMsg, keys []string) bool {
	for _, k := range keys {
		if msg.String() == k {
			return true
		}
	}
	return false
}
