package organizer

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	testDataDir         = "test_data"
	testImagesDir       = "sample_source" // Standard Go test data directory
	numTestFiles        = 100             // sCan be changed as needed
	minimalJPEGWithEXIF = "\xff\xd8\xff\xe1\x00\x42\x45\x78\x69\x66\x00\x00\x49\x49\x2a\x00\x08\x00\x00\x00\x01\x00\x9a\x82\x05\x00\x01\x00\x00\x00\x1a\x00\x00\x00\x00\x00\x00\x00\x32\x30\x32\x30\x3a\x30\x31\x3a\x30\x31\x20\x30\x30\x3a\x30\x30\x3a\x30\x30\x00\xff\xdb\x00\x43\x00\xff\xc0\x00\x0b\x08\x00\x01\x00\x01\x01\x01\x11\x00\xff\xc4\x00\x14\x00\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x03\xff\xc4\x00\x14\x10\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\xff\xda\x00\x08\x01\x01\x00\x00\x3f\x00\x7f\xff\xd9"
)

// TestMain handles setup and teardown for all tests
func TestMain(m *testing.M) {
	// Clean up any previous test data
	os.RemoveAll(testDataDir)

	// Setup fresh directory
	err := os.MkdirAll(testDataDir, 0755)
	if err != nil {
		fmt.Printf("Failed to create test directory: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()
	os.Exit(code)
}

// Add this function to generate a valid JPEG with EXIF
func generateJPEGWithEXIF(dateTime string) []byte {
	var buf bytes.Buffer

	// JPEG SOI
	buf.Write([]byte{0xFF, 0xD8})

	// APP1 EXIF
	exif := []byte{
		0xFF, 0xE1, // APP1 marker
		0x00, 0x00, // Length (placeholder)
		// EXIF header
		0x45, 0x78, 0x69, 0x66, 0x00, 0x00, // "Exif\0\0"
		// TIFF header
		0x4D, 0x4D, // Big endian
		0x00, 0x2A, // TIFF magic
		0x00, 0x00, 0x00, 0x08, // Offset to first IFD
		// IFD
		0x00, 0x01, // 1 entry
		// Entry
		0x90, 0x03, // Tag (DateTime)
		0x00, 0x02, // Type (ASCII)
		0x00, 0x00, 0x00, 0x14, // Count (20 bytes)
		0x00, 0x00, 0x00, 0x26, // Offset to data
	}
	buf.Write(exif)
	buf.WriteString(dateTime)
	buf.WriteByte(0)

	// JPEG EOI
	buf.Write([]byte{0xFF, 0xD9})

	// Update length
	data := buf.Bytes()
	binary.BigEndian.PutUint16(data[2:], uint16(len(data)-2))

	return data
}

// Copy test images from testdata to test_data directory
func copyTestImage(t *testing.T, srcName, dstName string) {
	t.Logf("Copying %s to %s", srcName, dstName)
	data, err := os.ReadFile(filepath.Join(testImagesDir, srcName))
	if err != nil {
		t.Fatalf("Failed to read test image %s: %v", srcName, err)
	}

	dstPath := filepath.Join(testDataDir, dstName)
	err = os.WriteFile(dstPath, data, 0644)
	if err != nil {
		t.Fatalf("Failed to write test image %s: %v", dstPath, err)
	}
	t.Logf("Successfully copied to %s", dstPath)
}

func generateTestFiles(t *testing.T, n int) {
	t.Helper()

	// Read all files from the sample source directory
	files, err := os.ReadDir(testImagesDir)
	if err != nil {
		t.Fatalf("Failed to read sample source directory: %v", err)
	}

	// Filter for .jpg files
	var sampleImages []string
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".JPG" {
			sampleImages = append(sampleImages, file.Name())
		}
	}

	if len(sampleImages) == 0 {
		t.Fatalf("No .jpg files found in %s", testImagesDir)
	}

	for i := 0; i < n; i++ {
		srcImage := sampleImages[i%len(sampleImages)]
		dstImage := fmt.Sprintf("test_image_%d.jpg", i)
		copyTestImage(t, srcImage, dstImage)
	}
}

func TestGroupWorkerSequential(t *testing.T) {
	t.Logf("Starting sequential test with test directory: %s", testDataDir)
	t.Logf("Looking for sample images in: %s", testImagesDir)

	// Read all files from the sample source directory
	files, err := os.ReadDir(testImagesDir)
	if err != nil {
		t.Fatalf("Failed to read sample_source directory: %v", err)
	}

	// Filter for .jpg files and copy them
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".JPG" {
			copyTestImage(t, file.Name(), file.Name())
		}
	}

	t.Logf("Starting organizer")
	org := NewOrganizer(testDataDir, "ymd", "generated", "seq", "1", "copy")
	org.AddFileEntries(testDataDir)
	org.groupWorker(0, len(org.fEntries)-1)

	// Verify files were processed
	for _, entry := range org.fEntries {
		if _, err := os.Stat(entry.NewPath); os.IsNotExist(err) {
			t.Errorf("Expected file %s to be created", entry.NewPath)
		}
	}
}

func TestGroupWorkerConcurrent(t *testing.T) {
	generateTestFiles(t, numTestFiles)

	org := NewOrganizer(testDataDir, "ymd", "generated", "con", "1", "copy")
	org.AddFileEntries(testDataDir)

	org.groupWorker(0, len(org.fEntries)-1)

	// Verify files were processed
	for _, entry := range org.fEntries {
		if _, err := os.Stat(entry.NewPath); os.IsNotExist(err) {
			t.Errorf("Expected file %s to be created", entry.NewPath)
		}
	}
}

// helper function for benchmarks
func generateBenchmarkFiles(b *testing.B, n int) {
	for i := 0; i < n; i++ {
		randTime := time.Date(
			2020+rand.Intn(4),
			time.Month(1+rand.Intn(12)),
			1+rand.Intn(28),
			rand.Intn(24),
			rand.Intn(60),
			rand.Intn(60),
			0,
			time.UTC,
		)

		dateStr := randTime.Format("2006:01:02 15:04:05")
		filename := filepath.Join(testDataDir, fmt.Sprintf("test_image_%d.jpg", i))

		currentDateStr := dateStr
		readMediaInfoFunc = func(filePath string) string {
			if filePath == filename {
				return fmt.Sprintf(`{"DateTimeOriginal": "%s"}`, currentDateStr)
			}
			return ""
		}

		// Use the new function to generate valid JPEG with EXIF
		jpegData := generateJPEGWithEXIF(dateStr)
		err := os.WriteFile(filename, jpegData, 0644)
		if err != nil {
			b.Fatalf("Failed to create test file: %v", err)
		}
	}
}

func BenchmarkFileOrganizerSequential(b *testing.B) {
	if err := os.MkdirAll(testDataDir, 0755); err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(testDataDir)

	generateBenchmarkFiles(b, numTestFiles) // Use the new function here

	org := NewOrganizer(testDataDir, "ymd", "generated", "seq", "1", "copy")
	for i := 0; i < b.N; i++ {
		org.AddFileEntries(org.SrcPath)
		org.OrganizeFiles()
		org.Clear()
	}
}

func BenchmarkFileOrganizerConcurrent(b *testing.B) {
	if err := os.MkdirAll(testDataDir, 0755); err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(testDataDir)

	generateBenchmarkFiles(b, numTestFiles)

	org := NewOrganizer(testDataDir, "ymd", "generated", "con", "1", "copy")
	for i := 0; i < b.N; i++ {
		org.AddFileEntries(org.SrcPath)
		org.OrganizeFiles()
		org.Clear()
	}
}

func TestGenFolder(t *testing.T) {
	testPath := "test_folder"
	defer os.RemoveAll(testPath) // Clean up after test

	org := NewOrganizer("", "", "", "", "", "")
	if err := org.genFolder(testPath); err != nil {
		t.Fatalf("genFolder returned error: %v", err)
	}
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Errorf("Expected folder %s to be created", testPath)
	}
}

func TestAddFileEntries(t *testing.T) {
	testPath := "test_folder"
	os.Mkdir(testPath, os.ModePerm)
	defer os.RemoveAll(testPath) // Clean up after test

	// Override readMediaInfoFunc to return dummy JSON for testing.
	originalFunc := readMediaInfoFunc
	readMediaInfoFunc = func(filePath string) string {
		return `{"DateTimeOriginal": "2020:01:01 00:00:00"}`
	}
	defer func() { readMediaInfoFunc = originalFunc }()

	// Create a dummy file with supported extension.
	testFile := filepath.Join(testPath, "test_image.jpg")
	file, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	file.Close()

	org := NewOrganizer(testPath, "ymd", "generated", "", "0", "")
	org.AddFileEntries(testPath)
	if len(org.fEntries) != 1 {
		t.Errorf("Expected 1 file entry, got %d", len(org.fEntries))
	}
}

func TestReadMediaInfo(t *testing.T) {
	// Override readMediaInfoFunc for testing.
	originalFunc := readMediaInfoFunc
	readMediaInfoFunc = func(filePath string) string {
		return `{"DateTimeOriginal": "2020:01:01 00:00:00"}`
	}
	defer func() { readMediaInfoFunc = originalFunc }()

	// Create a dummy file for testing with supported extension.
	testFilePath := "test_image.jpg"
	file, err := os.Create(testFilePath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	file.Close()
	defer os.Remove(testFilePath) // Clean up after test

	org := NewOrganizer("", "", "", "", "1", "")
	info := org.readMediaInfo(testFilePath)
	if info == "" {
		t.Errorf("Expected EXIF data, got empty string")
	}
}

func TestCopy(t *testing.T) {
	src := "src.txt"
	dst := "dst.txt"
	content := []byte("Hello, World!")

	// Create source file.
	err := os.WriteFile(src, content, 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}
	defer os.Remove(src) // Clean up after test

	org := NewOrganizer("", "", "", "", "1", "copy")
	_, err = org.copy(src, dst)
	if err != nil {
		t.Fatalf("Failed to copy file: %v", err)
	}
	defer os.Remove(dst) // Clean up after test

	// Verify content.
	dstContent, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}
	if string(dstContent) != string(content) {
		t.Errorf("Expected content %s, got %s", string(content), string(dstContent))
	}
}

func TestMove(t *testing.T) {
	src := "src.txt"
	dst := "dst.txt"
	content := []byte("Hello, World!")

	// Create source file.
	err := os.WriteFile(src, content, 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}
	defer os.Remove(src) // Clean up after test

	org := NewOrganizer("", "", "", "", "1", "move")
	_, err = org.move(src, dst)
	if err != nil {
		t.Fatalf("Failed to move file: %v", err)
	}
	defer os.Remove(dst) // Clean up after test

	// Verify content.
	dstContent, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}
	if string(dstContent) != string(content) {
		t.Errorf("Expected content %s, got %s", string(content), string(dstContent))
	}
}
