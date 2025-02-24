package main

import (
	"os"

	"github.com/developertyrone/picgroup/pkg/organizer"
)

func main() {
	// Execute sets up flags, instantiates the Organizer and runs it.
	organizer.Execute()

	// Optionally you might want to exit with a non-zero code on error.
	os.Exit(0)
}
