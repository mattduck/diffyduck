package cmd

import (
	"github.com/mattduck/diffyduck/v2/app"
	"github.com/mattduck/diffyduck/v2/renderer"
)

// RunV2 runs the v2 version of the application
func RunV2() error {
	// Create a real tview application
	tviewApp := renderer.NewTviewApplication()

	// Create and run the hello app
	helloApp := app.NewHelloApp(tviewApp)
	return helloApp.Run()
}
