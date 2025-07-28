package app

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/mattduck/diffyduck/v2/internal"
	"github.com/mattduck/diffyduck/v2/models"
	"github.com/mattduck/diffyduck/v2/ui"
)

// POCApp demonstrates the virtual viewport performance
type POCApp struct {
	screen   tcell.Screen
	viewport *ui.DiffViewport
	content  *models.DiffContent
	quit     bool

	// Background parsing state
	backgroundParsingComplete bool
}

// NewPOCApp creates a new POC application
func NewPOCApp(files []models.FileWithLines) (*POCApp, error) {
	internal.Log("[STARTUP] Creating POC app...")

	screen, err := tcell.NewScreen()
	if err != nil {
		return nil, err
	}
	internal.Log("[STARTUP] Created tcell screen")

	if err := screen.Init(); err != nil {
		return nil, err
	}
	internal.Log("[STARTUP] Initialized tcell screen")

	content := models.NewDiffContent(files)
	internal.Log("[STARTUP] Created diff content model")

	viewport := ui.NewDiffViewport(content)
	internal.Log("[STARTUP] Created diff viewport")

	app := &POCApp{
		screen:   screen,
		viewport: viewport,
		content:  content,
	}

	internal.Log("[STARTUP] POC app creation complete")
	return app, nil
}

// Run starts the POC application
func (app *POCApp) Run() error {
	internal.Log("[STARTUP] Starting POC app run...")
	defer app.cleanup()

	// Set initial viewport size
	width, height := app.screen.Size()
	app.viewport.SetSize(width, height-2) // Reserve space for status line
	internal.Logf("[STARTUP] Set viewport size: %dx%d", width, height-2)

	internal.Log("[STARTUP] About to call first render...")
	app.render()
	internal.Log("[STARTUP] First render complete, entering event loop")

	for !app.quit {
		// Handle events immediately (highest priority)
		if app.screen.HasPendingEvent() {
			event := app.screen.PollEvent()
			if !app.handleEvent(event) {
				app.quit = true
			}
			app.render()
		} else {
			// Do incremental parsing when no user input
			if !app.backgroundParsingComplete {
				allDone := app.viewport.ParseNextFileInBackground()
				if allDone {
					app.backgroundParsingComplete = true
				}
				app.render() // Only render during parsing
			} else {
				// Sleep when idle and parsing complete
				time.Sleep(10 * time.Millisecond)
			}
		}
	}

	return nil
}

// handleEvent processes input events
func (app *POCApp) handleEvent(event tcell.Event) bool {
	switch ev := event.(type) {
	case *tcell.EventKey:
		switch ev.Key() {
		case tcell.KeyEsc, tcell.KeyCtrlC:
			return false
		case tcell.KeyRune:
			switch ev.Rune() {
			case 'q':
				return false
			case 'j':
				app.viewport.ScrollVertical(1)
			case 'k':
				app.viewport.ScrollVertical(-1)
			case 'h':
				app.viewport.ScrollHorizontal(-8)
			case 'l':
				app.viewport.ScrollHorizontal(8)
			case 'd':
				height := app.viewport.GetHeight()
				app.viewport.ScrollVertical(height / 2)
			case 'u':
				height := app.viewport.GetHeight()
				app.viewport.ScrollVertical(-height / 2)
			case 'g':
				app.viewport.ScrollVertical(-app.content.TotalLines)
			case 'G':
				app.viewport.ScrollVertical(app.content.TotalLines)
			case 'f', ' ':
				height := app.viewport.GetHeight()
				app.viewport.ScrollVertical(height)
			case 'b':
				height := app.viewport.GetHeight()
				app.viewport.ScrollVertical(-height)
			}
		case tcell.KeyDown:
			app.viewport.ScrollVertical(1)
		case tcell.KeyUp:
			app.viewport.ScrollVertical(-1)
		case tcell.KeyLeft:
			app.viewport.ScrollHorizontal(-8)
		case tcell.KeyRight:
			app.viewport.ScrollHorizontal(8)
		case tcell.KeyPgDn:
			height := app.viewport.GetHeight()
			app.viewport.ScrollVertical(height)
		case tcell.KeyPgUp:
			height := app.viewport.GetHeight()
			app.viewport.ScrollVertical(-height)
		}

	case *tcell.EventResize:
		app.screen.Sync()
		width, height := app.screen.Size()
		app.viewport.SetSize(width, height-2)
	}

	return true
}

// render draws the application
func (app *POCApp) render() {
	app.screen.Clear()

	// Render the viewport
	app.viewport.Render(app.screen)

	// Render status line
	app.renderStatusLine()

	internal.Log("[RENDER] About to call screen.Show() - TUI will be visible after this")
	app.screen.Show()
	internal.Log("[RENDER] screen.Show() complete - TUI is now visible")
}

// renderStatusLine draws the status and performance info
func (app *POCApp) renderStatusLine() {
	width, height := app.screen.Size()
	if height < 2 {
		return
	}

	// Get viewport stats
	renderTime, renderCount := app.viewport.GetRenderStats()

	// Get current viewport info
	_, screenHeight := app.screen.Size()
	viewportHeight := screenHeight - 2 // Account for status line
	offsetY, offsetX := app.viewport.GetOffsets()

	// Create status text with essential info
	statusText := fmt.Sprintf(
		"Total: %d | Visible: %d | Offset: Y:%d X:%d | Render: %v (%d) | h/l:scroll j/k:line q:quit",
		app.content.TotalLines,
		viewportHeight,
		offsetY,
		offsetX,
		renderTime,
		renderCount,
	)

	// Draw status line
	style := tcell.StyleDefault.Background(tcell.ColorNavy).Foreground(tcell.ColorWhite)
	for i, r := range statusText {
		if i >= width {
			break
		}
		app.screen.SetContent(i, height-1, r, nil, style)
	}

	// Fill remaining space
	for i := len(statusText); i < width; i++ {
		app.screen.SetContent(i, height-1, ' ', nil, style)
	}
}

// GetHeight returns the viewport height (for key handling)
func (app *POCApp) GetHeight() int {
	_, height := app.screen.Size()
	return height - 2 // Account for status line
}

// cleanup releases resources
func (app *POCApp) cleanup() {
	if app.viewport != nil {
		app.viewport.Close()
	}
	if app.screen != nil {
		app.screen.Fini()
	}
}
