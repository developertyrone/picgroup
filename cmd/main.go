package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/developertyrone/picgroup/pkg/organizer"
)

// These variables are set during build using ldflags
var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	// Define command-line flags
	versionFlag := flag.Bool("version", false, "Print version information")
	srcPath := flag.String("d", "", "Directory path (absolute path)")
	folderFormat := flag.String("f", "ymd", "Folder format (ymd/ym)")
	generated := flag.String("t", "generated", "Generated folder name")
	groupMode := flag.String("g", "move", "Grouping mode (move/copy)")
	copyMode := flag.String("m", "seq", "File copy mode (seq/con)")
	verboseMode := flag.String("v", "0", "Verbose mode (0/1)")
	workerCount := flag.Int("w", runtime.NumCPU(), "Number of worker threads (for concurrent mode)")

	flag.Parse()

	// Check if version flag was provided
	if *versionFlag {
		fmt.Printf("PicGroup version %s (built at %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	// Execute the organizer with parsed flag values
	organizer.Execute(*srcPath, *folderFormat, *generated, *copyMode, *verboseMode, *groupMode, *workerCount)
}
