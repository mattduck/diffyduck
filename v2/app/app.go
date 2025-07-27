package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/mattduck/diffyduck/v2/models"
	"github.com/mattduck/diffyduck/v2/renderer"
	"github.com/mattduck/diffyduck/v2/ui"
)

// HelloApp orchestrates the hello world application
type HelloApp struct {
	app    renderer.Application
	widget *ui.HelloWidget
}

// NewHelloApp creates a new HelloApp with the given application
func NewHelloApp(app renderer.Application) *HelloApp {
	message := models.NewMessage("Hello World")
	widget := ui.NewHelloWidget(message)

	return &HelloApp{
		app:    app,
		widget: widget,
	}
}

// Run starts the application
func (h *HelloApp) Run() error {
	// Set up the root widget
	h.app.SetRoot(h.widget.GetPrimitive(), true)

	// Set up input capture for quit handling
	h.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		ev, shouldQuit := h.widget.HandleInput(event)
		if shouldQuit {
			h.app.Stop()
			return nil
		}
		return ev
	})

	// Run the application
	return h.app.Run()
}
