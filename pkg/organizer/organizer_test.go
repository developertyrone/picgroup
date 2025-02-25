package organizer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testDataDir   = "test_data"
	testImagesDir = "sample_source"
	numTestFiles  = 100
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

	// Make sure we clean up after all tests
	defer os.RemoveAll(testDataDir)

	// Run tests
	code := m.Run()
	os.Exit(code)
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

	// Filter for generated sample images
	var sampleImages []string
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "generated_sample_") && strings.HasSuffix(file.Name(), ".JPG") {
			sampleImages = append(sampleImages, file.Name())
		}
	}

	if len(sampleImages) == 0 {
		t.Fatalf("No generated sample images found in %s", testImagesDir)
	}

	for i := 0; i < n; i++ {
		// Pick a sample image (cycling through availablez ones if needed)
		srcImage := sampleImages[i%len(sampleImages)]
		dstImage := fmt.Sprintf("test_image_%d.jpg", i)

		// Copy the sample image
		copyTestImage(t, srcImage, dstImage)
	}
}

func TestGroupWorkerSequential(t *testing.T) {
	t.Logf("Starting sequential test with test directory: %s", testDataDir)
	t.Logf("Looking for sample images in: %s", testImagesDir)

	// Clean up any previous test data
	os.RemoveAll(testDataDir)

	// Create test directory
	err := os.MkdirAll(testDataDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Clean up after this test
	defer os.RemoveAll(testDataDir)

	// Read all files from the sample source directory
	files, err := os.ReadDir(testImagesDir)
	if err != nil {
		t.Fatalf("Failed to read sample_source directory: %v", err)
	}

	// Filter for generated sample images and copy them
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "generated_sample_") && strings.HasSuffix(file.Name(), ".JPG") {
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
	// Clean up any previous test data
	os.RemoveAll(testDataDir)

	// Create test directory
	err := os.MkdirAll(testDataDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Clean up after this test
	defer os.RemoveAll(testDataDir)

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
	// Read all files from the sample source directory
	files, err := os.ReadDir(testImagesDir)
	if err != nil {
		b.Fatalf("Failed to read sample source directory: %v", err)
	}

	// Filter for generated sample images
	var sampleImages []string
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "generated_sample_") && strings.HasSuffix(file.Name(), ".JPG") {
			sampleImages = append(sampleImages, file.Name())
		}
	}

	if len(sampleImages) == 0 {
		b.Fatalf("No generated sample images found in %s", testImagesDir)
	}

	for i := 0; i < n; i++ {
		// Pick a sample image (cycling through available ones if needed)
		srcImage := sampleImages[i%len(sampleImages)]
		dstPath := filepath.Join(testDataDir, fmt.Sprintf("test_image_%d.jpg", i))

		// Copy the sample image
		srcPath := filepath.Join(testImagesDir, srcImage)
		data, err := os.ReadFile(srcPath)
		if err != nil {
			b.Fatalf("Failed to read test image %s: %v", srcPath, err)
		}

		err = os.WriteFile(dstPath, data, 0644)
		if err != nil {
			b.Fatalf("Failed to write test image %s: %v", dstPath, err)
		}
	}
}

func BenchmarkFileOrganizerSequential(b *testing.B) {
	if err := os.MkdirAll(testDataDir, 0755); err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(testDataDir)

	generateBenchmarkFiles(b, numTestFiles)

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

// The rest of the test functions stay the same
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

func TestReadMediaInfo(t *testing.T) {
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
	// Since this is an empty file, we expect no EXIF info
	if info != "" {
		t.Errorf("Expected empty string for file with no EXIF, got %s", info)
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
