package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/mattduck/diffyduck/v2/models"
	"github.com/rivo/tview"
)

// HelloWidget represents our hello world UI component
type HelloWidget struct {
	textView *tview.TextView
	message  *models.Message
}

// NewHelloWidget creates a new HelloWidget
func NewHelloWidget(message *models.Message) *HelloWidget {
	textView := tview.NewTextView().
		SetText(message.GetText()).
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	return &HelloWidget{
		textView: textView,
		message:  message,
	}
}

// GetPrimitive returns the underlying tview primitive
func (h *HelloWidget) GetPrimitive() tview.Primitive {
	return h.textView
}

// HandleInput processes keyboard input
func (h *HelloWidget) HandleInput(event *tcell.EventKey) (*tcell.EventKey, bool) {
	if event.Key() == tcell.KeyRune && event.Rune() == 'q' {
		return nil, true // Signal to quit
	}
	return event, false
}
