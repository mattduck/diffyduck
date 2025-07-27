package app

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/mattduck/diffyduck/v2/models"
	"github.com/mattduck/diffyduck/v2/ui"
)

// POCApp demonstrates the virtual viewport performance
type POCApp struct {
	screen   tcell.Screen
	viewport *ui.DiffViewport
	content  *models.DiffContent
	quit     bool

	// Stats
	frameCount    int
	lastStatsTime time.Time
	fps           float64
}

// NewPOCApp creates a new POC application
func NewPOCApp(files []models.FileWithLines) (*POCApp, error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		return nil, err
	}

	if err := screen.Init(); err != nil {
		return nil, err
	}

	content := models.NewDiffContent(files)
	viewport := ui.NewDiffViewport(content)

	app := &POCApp{
		screen:        screen,
		viewport:      viewport,
		content:       content,
		lastStatsTime: time.Now(),
	}

	return app, nil
}

// Run starts the POC application
func (app *POCApp) Run() error {
	defer app.cleanup()

	// Set initial viewport size
	width, height := app.screen.Size()
	app.viewport.SetSize(width, height-2) // Reserve space for status line

	// Main event loop - process events immediately for responsiveness
	statsTicker := time.NewTicker(time.Second) // Update stats every second
	defer statsTicker.Stop()

	app.render()

	for !app.quit {
		select {
		case <-statsTicker.C:
			// Update stats periodically
			app.updateStats()
			app.render() // Re-render to show updated stats

		default:
			// Check for events immediately
			if app.screen.HasPendingEvent() {
				event := app.screen.PollEvent()
				if !app.handleEvent(event) {
					app.quit = true
				}
				// Render immediately after handling event
				app.render()
			} else {
				// Small sleep to prevent busy waiting
				time.Sleep(1 * time.Millisecond)
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

	app.screen.Show()
	app.frameCount++
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

	// Create status text with more detailed info
	statusText := fmt.Sprintf(
		"Total: %d | Visible: %d | Offset: Y:%d X:%d | FPS: %.1f | Render: %v (%d) | h/l:scroll j/k:line q:quit",
		app.content.TotalLines,
		viewportHeight,
		offsetY,
		offsetX,
		app.fps,
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

// updateStats calculates FPS and other performance metrics
func (app *POCApp) updateStats() {
	now := time.Now()
	if time.Since(app.lastStatsTime) >= time.Second {
		duration := now.Sub(app.lastStatsTime)
		app.fps = float64(app.frameCount) / duration.Seconds()
		app.frameCount = 0
		app.lastStatsTime = now
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
