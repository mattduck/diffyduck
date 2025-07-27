package renderer

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Screen is an abstraction over tcell.Screen to enable testing
type Screen interface {
	// Core screen operations
	Init() error
	Fini()
	Clear()
	Show()
	Size() (width, height int)

	// Event handling
	PollEvent() tcell.Event
	PostEvent(ev tcell.Event) error

	// Content operations
	SetContent(x, y int, primary rune, combining []rune, style tcell.Style)
	GetContent(x, y int) (primary rune, combining []rune, style tcell.Style, width int)
}

// Application is an abstraction over tview.Application to enable testing
type Application interface {
	SetRoot(root tview.Primitive, fullscreen bool) *tview.Application
	SetInputCapture(capture func(event *tcell.EventKey) *tcell.EventKey) *tview.Application
	Run() error
	Stop()
	Draw()
	GetScreen() tcell.Screen
}

// TviewApplication wraps tview.Application to implement our Application interface
type TviewApplication struct {
	*tview.Application
}

// NewTviewApplication creates a new TviewApplication
func NewTviewApplication() *TviewApplication {
	return &TviewApplication{
		Application: tview.NewApplication(),
	}
}

// Draw implements the Application interface
func (t *TviewApplication) Draw() {
	t.Application.Draw()
}

// GetScreen returns the underlying tcell.Screen
func (t *TviewApplication) GetScreen() tcell.Screen {
	// tview doesn't expose GetScreen directly, we'll need to work around this
	// For now, return nil as we won't use this in our simple hello world example
	return nil
}
