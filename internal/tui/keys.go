package tui

import "github.com/charmbracelet/bubbletea"

// KeyMap defines the key bindings for the application.
type KeyMap struct {
	Up            []string
	Down          []string
	PageUp        []string
	PageDown      []string
	HalfUp        []string
	HalfDown      []string
	Top           []string
	Bottom        []string
	Left          []string
	Right         []string
	Quit          []string
	SearchForward []string
	SearchBack    []string
	NextMatch     []string
	PrevMatch     []string
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:            []string{"up", "k"},
		Down:          []string{"down", "j"},
		PageUp:        []string{"pgup", "ctrl+b", "b"},
		PageDown:      []string{"pgdown", "ctrl+f", " ", "f"},
		HalfUp:        []string{"ctrl+u"},
		HalfDown:      []string{"ctrl+d"},
		Top:           []string{"home", "g"},
		Bottom:        []string{"end", "G"},
		Left:          []string{"left", "h"},
		Right:         []string{"right", "l"},
		Quit:          []string{"q", "ctrl+c"},
		SearchForward: []string{"/"},
		SearchBack:    []string{"?"},
		NextMatch:     []string{"n"},
		PrevMatch:     []string{"N"},
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
