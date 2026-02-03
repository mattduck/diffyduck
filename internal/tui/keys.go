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
	}
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
