package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/mattduck/diffyduck/v2/models"
	"github.com/stretchr/testify/assert"
)

func TestNewHelloWidget(t *testing.T) {
	message := models.NewMessage("Test Message")
	widget := NewHelloWidget(message)

	assert.NotNil(t, widget)
	assert.NotNil(t, widget.textView)
	assert.Equal(t, message, widget.message)
	assert.NotNil(t, widget.GetPrimitive())
}

func TestHelloWidget_HandleInput(t *testing.T) {
	message := models.NewMessage("Test")
	widget := NewHelloWidget(message)

	tests := []struct {
		name        string
		event       *tcell.EventKey
		wantQuit    bool
		wantHandled bool
	}{
		{
			name:        "quit on 'q'",
			event:       tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone),
			wantQuit:    true,
			wantHandled: true,
		},
		{
			name:        "pass through other keys",
			event:       tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone),
			wantQuit:    false,
			wantHandled: false,
		},
		{
			name:        "pass through special keys",
			event:       tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone),
			wantQuit:    false,
			wantHandled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			returnedEvent, shouldQuit := widget.HandleInput(tt.event)

			assert.Equal(t, tt.wantQuit, shouldQuit)
			if tt.wantHandled {
				assert.Nil(t, returnedEvent)
			} else {
				assert.Equal(t, tt.event, returnedEvent)
			}
		})
	}
}
