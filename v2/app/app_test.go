package app

import (
	"errors"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/mattduck/diffyduck/v2/internal/mocks"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewHelloApp(t *testing.T) {
	mockApp := new(mocks.MockApplication)
	app := NewHelloApp(mockApp)

	assert.NotNil(t, app)
	assert.NotNil(t, app.app)
	assert.NotNil(t, app.widget)
}

func TestHelloApp_Run(t *testing.T) {
	tests := []struct {
		name    string
		runErr  error
		wantErr bool
	}{
		{
			name:    "successful run",
			runErr:  nil,
			wantErr: false,
		},
		{
			name:    "run error",
			runErr:  errors.New("run failed"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockApp := new(mocks.MockApplication)
			app := NewHelloApp(mockApp)

			// Set expectations
			mockApp.On("SetRoot", mock.Anything, true).Return(&tview.Application{})
			mockApp.On("SetInputCapture", mock.Anything).Return(&tview.Application{})
			mockApp.On("Run").Return(tt.runErr)

			// Run the app
			err := app.Run()

			// Verify
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tt.runErr, err)
			} else {
				assert.NoError(t, err)
			}

			mockApp.AssertExpectations(t)
		})
	}
}

func TestHelloApp_InputHandling(t *testing.T) {
	mockApp := new(mocks.MockApplication)
	app := NewHelloApp(mockApp)

	var capturedInputHandler func(event *tcell.EventKey) *tcell.EventKey

	// Set expectations
	mockApp.On("SetRoot", mock.Anything, true).Return(&tview.Application{})
	mockApp.On("SetInputCapture", mock.Anything).Run(func(args mock.Arguments) {
		capturedInputHandler = args.Get(0).(func(event *tcell.EventKey) *tcell.EventKey)
	}).Return(&tview.Application{})
	mockApp.On("Stop").Return()

	// Start setup (but don't run)
	app.app.SetRoot(app.widget.GetPrimitive(), true)
	app.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		ev, shouldQuit := app.widget.HandleInput(event)
		if shouldQuit {
			app.app.Stop()
			return nil
		}
		return ev
	})

	// Test quit on 'q'
	qEvent := tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone)
	result := capturedInputHandler(qEvent)
	assert.Nil(t, result)
	mockApp.AssertCalled(t, "Stop")

	// Test pass-through for other keys
	aEvent := tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone)
	result = capturedInputHandler(aEvent)
	assert.Equal(t, aEvent, result)

	mockApp.AssertExpectations(t)
}
