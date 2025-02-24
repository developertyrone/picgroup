package organizer

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/tidwall/gjson"
)

// --- Overridable EXIF reader function (used for testing) ---

var readMediaInfoFunc = defaultReadMediaInfo

func defaultReadMediaInfo(filePath string) string {
	ext := strings.ToUpper(filepath.Ext(filePath))
	switch ext {
	case ".JPG", ".JPEG", ".PNG", ".ARW":
		imgFile, err := os.Open(filePath)
		if err != nil {
			log.Println(err.Error())
			return ""
		}
		defer imgFile.Close()

		metaData, err := exif.Decode(imgFile)
		if err != nil {
			log.Println(err.Error())
			return ""
		}

		jsonByte, err := metaData.MarshalJSON()
		if err != nil {
			log.Println(err.Error())
			return ""
		}
		return string(jsonByte)
	default:
		return ""
	}
}

// --- Core types and constructor ---

// FileData holds information about a file entry to be organized.
type FileData struct {
	Path    string
	Info    os.FileInfo
	NewPath string
}

// Organizer holds configuration and state for organizing files.
type Organizer struct {
	SrcPath      string
	FolderFormat string
	Generated    string
	CopyMode     string
	VerboseMode  string
	GroupMode    string

	fEntries    []FileData
	dateFolders map[string]bool
}

// NewOrganizer creates a new Organizer instance with the given parameters.
func NewOrganizer(srcPath, folderFormat, generated, copyMode, verboseMode, groupMode string) *Organizer {
	return &Organizer{
		SrcPath:      srcPath,
		FolderFormat: folderFormat,
		Generated:    generated,
		CopyMode:     copyMode,
		VerboseMode:  verboseMode,
		GroupMode:    groupMode,
		fEntries:     make([]FileData, 0),
		dateFolders:  make(map[string]bool),
	}
}

// Execute parses the command-line flags, creates an Organizer, and runs the organization process.
func Execute() {
	// Command-line flags.
	srcPathInput := flag.String("d", ".", "Directory path (absolute path)")
	formatInput := flag.String("f", "ymd", "Folder format (ymd/ym)")
	generatedInput := flag.String("t", "generated", "Generated folder name")
	groupModeInput := flag.String("g", "move", "Grouping mode (move/copy)")
	copyModeInput := flag.String("m", "seq", "File copy mode (seq/con)")
	verboseInput := flag.String("v", "0", "Verbose mode (0/1)")

	flag.Parse()

	if *srcPathInput == "." {
		fmt.Println("Please define a path with -d and -h for help")
		os.Exit(0)
	}

	org := NewOrganizer(*srcPathInput, *formatInput, *generatedInput, *copyModeInput, *verboseInput, *groupModeInput)

	if org.VerboseMode == "1" {
		defer trackTime(time.Now(), "process")
	}

	org.AddFileEntries(org.SrcPath)
	org.OrganizeFiles()
	org.Clear()
}

// Clear resets the Organizer's state.
func (o *Organizer) Clear() {
	o.dateFolders = make(map[string]bool)
	o.fEntries = make([]FileData, 0)
}

// AddFileEntries recursively scans the given directory and adds eligible file entries.
func (o *Organizer) AddFileEntries(fromPath string) {
	entries, err := os.ReadDir(fromPath)
	if err != nil {
		panic(err)
	}

	if o.VerboseMode == "1" {
		fmt.Printf("%s has %d files\n", fromPath, len(entries))
	}

	for _, entry := range entries {
		fullPath := path.Join(fromPath, entry.Name())
		if entry.IsDir() {
			// Avoid rescanning the generated folder or hidden directories.
			if entry.Name() != o.Generated && !strings.HasPrefix(entry.Name(), ".") && !strings.HasPrefix(entry.Name(), "@") {
				o.AddFileEntries(fullPath)
			}
		} else {
			fileInfo, err := entry.Info()
			if err != nil {
				log.Println("Failed to get file info:", err)
				continue
			}

			infoStr := o.readMediaInfo(fullPath)
			log.Printf("infoStr:%s\n %s\n", fullPath, infoStr)
			if infoStr != "" {
				dateTimeOriginal := gjson.Get(infoStr, "DateTimeOriginal").String()
				createTime, err := time.Parse("2006:01:02 15:04:05", dateTimeOriginal)
				if err != nil {
					log.Println("Failed to parse date:", err)
					continue
				}

				var newFolder string
				switch o.FolderFormat {
				case "ymd":
					newFolder = createTime.Format("20060102")
				case "ym":
					newFolder = createTime.Format("200601")
				default:
					newFolder = createTime.Format("20060102")
				}

				o.fEntries = append(o.fEntries, FileData{
					Path:    fullPath,
					Info:    fileInfo,
					NewPath: path.Join(o.SrcPath, o.Generated, newFolder, filepath.Base(fullPath)),
				})
				o.dateFolders[newFolder] = true
			}
		}
	}
}

// OrganizeFiles creates the necessary folders and processes file entries (either sequentially or concurrently).
func (o *Organizer) OrganizeFiles() {
	if o.VerboseMode == "2" {
		fmt.Printf("Generated folders: %d\n", len(o.dateFolders))
		fmt.Printf("All processed file entries: %d\n", len(o.fEntries))
	}

	if len(o.dateFolders) > 0 {
		// Create the main generated folder.
		if err := o.genFolder(o.SrcPath, o.Generated); err != nil {
			log.Printf("Error creating folder: %v", err)
			return
		}

		// Create subfolders for each date.
		for dateKey := range o.dateFolders {
			if err := o.genFolder(o.SrcPath, o.Generated, dateKey); err != nil {
				log.Printf("Error creating subfolder: %v", err)
			}
		}

		switch o.CopyMode {
		case "seq":
			o.groupWorker(0, len(o.fEntries)-1)
		case "con":
			numWorkers := maxParallelism()
			if o.VerboseMode == "1" {
				fmt.Println("Number of workers:", numWorkers)
			}
			var wg sync.WaitGroup
			segment := len(o.fEntries) / numWorkers
			for i := 0; i <= len(o.fEntries)/numWorkers; i++ {
				start := i * segment
				end := (i+1)*segment - 1
				if end >= len(o.fEntries) {
					end = len(o.fEntries) - 1
				}
				wg.Add(1)
				go func(routine, startIdx, endIdx int) {
					defer wg.Done()
					if o.VerboseMode == "1" {
						fmt.Printf("Worker %d processing from %d to %d\n", routine, startIdx, endIdx)
					}
					o.groupWorker(startIdx, endIdx)
				}(i, start, end)
			}
			wg.Wait()
		}

		if o.VerboseMode == "1" {
			fmt.Println("Processed file entries count:", len(o.fEntries))
		}
	}
}

// genFolder creates a folder from the given path components.
func (o *Organizer) genFolder(paths ...string) error {
	fullPath := path.Join(paths...)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		err := os.Mkdir(fullPath, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}

// readMediaInfo obtains the media file's EXIF information via the overridable readMediaInfoFunc.
func (o *Organizer) readMediaInfo(filePath string) string {
	result := readMediaInfoFunc(filePath)
	if result == "" && o.VerboseMode == "1" {
		fmt.Println(strings.ToUpper(filepath.Ext(filePath)), "file not supported or invalid EXIF")
	}
	return result
}

// groupWorker processes file entries in the index range from start to end.
func (o *Organizer) groupWorker(start, end int) {
	for i := start; i <= end; i++ {
		if i < len(o.fEntries) {
			if o.VerboseMode == "1" {
				fmt.Printf("%s processing file %d: %s -> %s\n", o.GroupMode, i, o.fEntries[i].Path, o.fEntries[i].NewPath)
			}
			switch o.GroupMode {
			case "copy":
				_, err := o.copy(o.fEntries[i].Path, o.fEntries[i].NewPath)
				if err != nil {
					log.Printf("Error copying file: %v", err)
				}
			case "move":
				_, err := o.move(o.fEntries[i].Path, o.fEntries[i].NewPath)
				if err != nil {
					log.Printf("Error moving file: %v", err)
				}
			}
		}
	}
}

// copy copies a file from src to dst.
func (o *Organizer) copy(src, dst string) (int64, error) {
	// Create destination directory if it doesn't exist
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create destination directory: %v", err)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()

	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

// move moves a file from src to dst.
func (o *Organizer) move(src, dst string) (int64, error) {
	err := os.Rename(src, dst)
	if err != nil {
		return 0, err
	}
	return 1, nil
}

// maxParallelism returns an appropriate number of parallel workers.
func maxParallelism() int {
	maxProcs := runtime.GOMAXPROCS(0)
	numCPU := runtime.NumCPU()
	if maxProcs < numCPU {
		return maxProcs
	}
	return numCPU
}

// trackTime logs the elapsed time since start.
func trackTime(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("%s took %s", name, elapsed)
}
