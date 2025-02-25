package organizer

import (
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

	"github.com/dsoprea/go-exif/v3"
	"github.com/tidwall/gjson"

	"encoding/json"
	"runtime/debug"
)

// --- Overridable EXIF reader function (used for testing) ---

var readMediaInfoFunc = defaultReadMediaInfo

func defaultReadMediaInfo(filePath string) string {
	ext := strings.ToUpper(filepath.Ext(filePath))
	switch ext {
	case ".JPG", ".JPEG", ".PNG", ".ARW":
		rawExif, err := exif.SearchFileAndExtractExif(filePath)
		if err != nil {
			log.Printf("No EXIF in %s: %v", filePath, err)
			return ""
		}
		flatExif, _, err := exif.GetFlatExifData(rawExif, nil)
		if err != nil {
			log.Printf("Error retrieving EXIF data for %s: %v", filePath, err)
			return ""
		}
		metadata := make(map[string]string)
		// Look for DateTimeOriginal first.
		for _, item := range flatExif {
			if item.TagName == "DateTimeOriginal" {
				metadata["DateTimeOriginal"] = item.FormattedFirst
				break
			}
		}
		// Fallback to DateTime if DateTimeOriginal is not available.
		if metadata["DateTimeOriginal"] == "" {
			for _, item := range flatExif {
				if item.TagName == "DateTime" {
					metadata["DateTimeOriginal"] = item.FormattedFirst
					break
				}
			}
		}
		jsonBytes, err := json.Marshal(metadata)
		if err != nil {
			log.Printf("Error marshaling EXIF metadata for %s: %v", filePath, err)
			return ""
		}
		return string(jsonBytes)
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

// Execute creates an Organizer with the provided parameters and runs the organization process.
func Execute(srcPath, folderFormat, generated, copyMode, verboseMode, groupMode string, workerCount int) {
	if srcPath == "" {
		fmt.Println("Please define a valid path")
		os.Exit(0)
	}

	// Disable GC during heavy processing
	debug.SetGCPercent(-1)
	defer debug.SetGCPercent(100)

	org := NewOrganizer(srcPath, folderFormat, generated, copyMode, verboseMode, groupMode)

	if org.VerboseMode == "1" {
		defer trackTime(time.Now(), "process")
	}

	org.AddFileEntries(org.SrcPath)
	org.OrganizeFiles(workerCount)
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
func (o *Organizer) OrganizeFiles(customWorkerCount int) {
	if o.VerboseMode == "2" {
		fmt.Printf("Generated folders: %d\n", len(o.dateFolders))
		fmt.Printf("All processed file entries: %d\n", len(o.fEntries))
	}

	if len(o.dateFolders) == 0 {
		return
	}

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
		numWorkers := customWorkerCount
		if numWorkers <= 0 {
			numWorkers = maxParallelism()
		}

		// Ensure we don't create more workers than we have files
		if numWorkers > len(o.fEntries) {
			numWorkers = len(o.fEntries)
		}

		if o.VerboseMode == "1" {
			fmt.Println("Number of workers:", numWorkers)
		}

		// Create work queue and semaphore for limiting concurrent disk operations
		fileQueue := make(chan FileData, len(o.fEntries))
		semaphore := make(chan struct{}, numWorkers*2) // Allow twice as many queued operations

		// Fill the work queue with all files
		for _, fileEntry := range o.fEntries {
			fileQueue <- fileEntry
		}
		close(fileQueue)

		// Create a WaitGroup to wait for all workers
		var wg sync.WaitGroup
		wg.Add(numWorkers)

		// Start workers
		for i := 0; i < numWorkers; i++ {
			go func(workerID int) {
				defer wg.Done()
				processedCount := 0
				startTime := time.Now()

				// Process files from the queue
				for fileEntry := range fileQueue {
					// Acquire semaphore slot
					semaphore <- struct{}{}

					if o.VerboseMode == "1" && processedCount%100 == 0 && processedCount > 0 {
						elapsed := time.Since(startTime)
						fmt.Printf("Worker %d processed %d files (%.2f files/sec)\n",
							workerID, processedCount, float64(processedCount)/elapsed.Seconds())
					}

					// Process the file
					o.processFile(fileEntry)
					processedCount++

					// Release semaphore slot
					<-semaphore
				}

				if o.VerboseMode == "1" {
					elapsed := time.Since(startTime)
					fmt.Printf("Worker %d finished - processed %d files in %s (%.2f files/sec)\n",
						workerID, processedCount, elapsed, float64(processedCount)/elapsed.Seconds())
				}
			}(i)
		}

		// Wait for all workers to complete
		wg.Wait()
	}

	if o.VerboseMode == "1" {
		fmt.Println("Processed file entries count:", len(o.fEntries))
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

// processFile processes a single file entry according to the group mode
func (o *Organizer) processFile(fileEntry FileData) {
	if o.VerboseMode == "1" {
		fmt.Printf("%s processing file: %s -> %s\n", o.GroupMode, fileEntry.Path, fileEntry.NewPath)
	}

	switch o.GroupMode {
	case "copy":
		_, err := o.copy(fileEntry.Path, fileEntry.NewPath)
		if err != nil {
			log.Printf("Error copying file: %v", err)
		}
	case "move":
		_, err := o.move(fileEntry.Path, fileEntry.NewPath)
		if err != nil {
			log.Printf("Error moving file: %v", err)
		}
	}
}

// copy copies a file from src to dst with buffered I/O for performance
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

	// Use a larger buffer for better performance
	buf := make([]byte, 1024*1024) // 1MB buffer
	nBytes, err := io.CopyBuffer(destination, source, buf)
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
