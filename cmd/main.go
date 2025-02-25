package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/developertyrone/picgroup/pkg/organizer"
)

// These variables are set during build using ldflags
var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	// Add version flag
	versionFlag := flag.Bool("version", false, "Print version information")
	flag.Parse()

	// Check if version flag was provided
	if *versionFlag {
		fmt.Printf("PicGroup version %s (built at %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	// Execute sets up flags, instantiates the Organizer and runs it.
	organizer.Execute()

	// Optionally you might want to exit with a non-zero code on error.
	os.Exit(0)
}
