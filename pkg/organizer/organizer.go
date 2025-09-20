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
			return ""
		}
		flatExif, _, err := exif.GetFlatExifData(rawExif, nil)
		if err != nil {
			return ""
		}
		
		// Only extract the date we need, don't store all metadata
		var dateTimeOriginal string
		for _, item := range flatExif {
			if item.TagName == "DateTimeOriginal" {
				dateTimeOriginal = item.FormattedFirst
				break
			}
		}
		// Fallback to DateTime if DateTimeOriginal is not available
		if dateTimeOriginal == "" {
			for _, item := range flatExif {
				if item.TagName == "DateTime" {
					dateTimeOriginal = item.FormattedFirst
					break
				}
			}
		}
		
		if dateTimeOriginal != "" {
			// Return minimal JSON with just the date
			return fmt.Sprintf(`{"DateTimeOriginal":"%s"}`, dateTimeOriginal)
		}
		return ""
	default:
		return ""
	}
}

// --- Core types and constructor ---

// FileData holds minimal information about a file entry to be organized.
type FileData struct {
	Path    string
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

	// Limit CPU usage to prevent macOS security from killing the process
	runtime.GOMAXPROCS(4)

	// Keep GC enabled but tune it for better memory management
	debug.SetGCPercent(20) // More aggressive GC
	defer debug.SetGCPercent(100)

	org := NewOrganizer(srcPath, folderFormat, generated, copyMode, verboseMode, groupMode)

	if org.VerboseMode == "1" {
		defer trackTime(time.Now(), "process")
	}

	// First pass: scan and create folders
	org.ScanAndCreateFolders(org.SrcPath)
	
	// Second pass: process files in streaming batches
	org.ProcessFilesInBatches(org.SrcPath, workerCount)
	
	org.Clear()
}

// Clear resets the Organizer's state.
func (o *Organizer) Clear() {
	o.dateFolders = make(map[string]bool)
	o.fEntries = make([]FileData, 0)
}

// ScanAndCreateFolders does a lightweight scan to create all needed folders without loading files into memory
func (o *Organizer) ScanAndCreateFolders(fromPath string) {
	o.scanForDateFolders(fromPath)
	
	// Create the main generated folder
	if err := o.genFolder(o.SrcPath, o.Generated); err != nil {
		log.Printf("Error creating main folder: %v", err)
		return
	}

	// Create subfolders for each date
	for dateKey := range o.dateFolders {
		if err := o.genFolder(o.SrcPath, o.Generated, dateKey); err != nil {
			log.Printf("Error creating subfolder: %v", err)
		}
	}
	
	if o.VerboseMode == "1" {
		fmt.Printf("Created %d date folders\n", len(o.dateFolders))
	}
}

// scanForDateFolders recursively scans to find date folders needed, without storing file data
func (o *Organizer) scanForDateFolders(fromPath string) {
	entries, err := os.ReadDir(fromPath)
	if err != nil {
		log.Printf("Error reading directory %s: %v", fromPath, err)
		return
	}

	for _, entry := range entries {
		fullPath := path.Join(fromPath, entry.Name())
		if entry.IsDir() {
			if entry.Name() != o.Generated && !strings.HasPrefix(entry.Name(), ".") && !strings.HasPrefix(entry.Name(), "@") {
				o.scanForDateFolders(fullPath)
			}
		} else {
			// We don't need fileInfo, just check if we can read EXIF
			infoStr := o.readMediaInfo(fullPath)
			if infoStr != "" {
				dateTimeOriginal := gjson.Get(infoStr, "DateTimeOriginal").String()
				createTime, err := time.Parse("2006:01:02 15:04:05", dateTimeOriginal)
				if err != nil {
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
				o.dateFolders[newFolder] = true
			}
		}
	}
}
// ProcessFilesInBatches processes files in small batches to keep memory usage low
func (o *Organizer) ProcessFilesInBatches(fromPath string, workerCount int) {
	const batchSize = 100 // Small batch size to keep memory low
	
	o.processDirectoryInBatches(fromPath, batchSize, workerCount)
}

// processDirectoryInBatches recursively processes directories in small batches
func (o *Organizer) processDirectoryInBatches(fromPath string, batchSize, workerCount int) {
	entries, err := os.ReadDir(fromPath)
	if err != nil {
		log.Printf("Error reading directory %s: %v", fromPath, err)
		return
	}

	if o.VerboseMode == "1" {
		fmt.Printf("Processing %s with %d entries\n", fromPath, len(entries))
	}

	batch := make([]FileData, 0, batchSize)
	
	for _, entry := range entries {
		fullPath := path.Join(fromPath, entry.Name())
		if entry.IsDir() {
			// Process current batch before recursing
			if len(batch) > 0 {
				o.processBatch(batch, workerCount)
				batch = batch[:0] // Clear batch
				runtime.GC() // Force GC between batches
			}
			
			if entry.Name() != o.Generated && !strings.HasPrefix(entry.Name(), ".") && !strings.HasPrefix(entry.Name(), "@") {
				o.processDirectoryInBatches(fullPath, batchSize, workerCount)
			}
		} else {
			// Only read EXIF metadata, don't store file info
			infoStr := o.readMediaInfo(fullPath)
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

				batch = append(batch, FileData{
					Path:    fullPath,
					NewPath: path.Join(o.SrcPath, o.Generated, newFolder, filepath.Base(fullPath)),
				})

				// Process batch when it reaches the size limit
				if len(batch) >= batchSize {
					o.processBatch(batch, workerCount)
					batch = batch[:0] // Clear batch
					runtime.GC() // Force GC between batches
				}
			}
		}
	}

	// Process remaining files in the final batch
	if len(batch) > 0 {
		o.processBatch(batch, workerCount)
		runtime.GC()
	}
}

// processBatch processes a small batch of files
func (o *Organizer) processBatch(batch []FileData, workerCount int) {
	if len(batch) == 0 {
		return
	}
	
	if o.VerboseMode == "1" {
		fmt.Printf("Processing batch of %d files\n", len(batch))
	}

	switch o.CopyMode {
	case "seq":
		for i := range batch {
			o.processFile(batch[i])
		}
	case "con":
		// Limit workers for small batches
		numWorkers := workerCount
		if numWorkers <= 0 {
			numWorkers = 2 // Use only 2 workers for batches
		}
		if numWorkers > len(batch) {
			numWorkers = len(batch)
		}
		if numWorkers > 4 { // Cap at 4 to prevent CPU overload
			numWorkers = 4
		}

		fileQueue := make(chan FileData, len(batch))
		var wg sync.WaitGroup

		// Fill queue
		for _, fileEntry := range batch {
			fileQueue <- fileEntry
		}
		close(fileQueue)

		// Start workers
		wg.Add(numWorkers)
		for i := 0; i < numWorkers; i++ {
			go func() {
				defer wg.Done()
				for fileEntry := range fileQueue {
					o.processFile(fileEntry)
					time.Sleep(2 * time.Millisecond) // Small delay to prevent CPU overload
				}
			}()
		}

		wg.Wait()
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
			// Cap workers to prevent maxing out CPU and triggering macOS security
			if numWorkers > 4 {
				numWorkers = 4
			}
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

					// Run manual GC every 100 files to manage memory
					if processedCount%100 == 0 {
						runtime.GC()
					}

					// Add small delay to prevent overwhelming CPU
					time.Sleep(1 * time.Millisecond)

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

	// Use a smaller buffer to reduce memory footprint
	buf := make([]byte, 64*1024) // 64KB buffer instead of 1MB
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
