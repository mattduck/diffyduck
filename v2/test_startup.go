package main

import (
	"fmt"
	"time"

	"github.com/mattduck/diffyduck/v2/cmd"
)

func main() {
	fmt.Println("Testing progressive parsing startup...")

	start := time.Now()

	// This will initialize the POC app with progressive parsing
	err := cmd.RunPOC()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// This won't be reached since RunPOC() runs the interactive app
	elapsed := time.Since(start)
	fmt.Printf("Total time: %v\n", elapsed)
}
