package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func BenchmarkFileOrganzier(b *testing.B) {

	parseInput("/Users/superkufu/Desktop/testimage2", "ymd", "generated", "seq", "1", "copy")
	addFileEntries(src_path, &fentries)
	orgnizeFiles()
	clear()
}

func BenchmarkConcurrentFileOrganzier(b *testing.B) {

	parseInput("/Users/superkufu/Desktop/testimage2", "ymd", "generated", "con", "1", "copy")
	addFileEntries(src_path, &fentries)
	orgnizeFiles()
	clear()
}

func TestGenFolder(t *testing.T) {
	testPath := "test_folder"
	defer os.RemoveAll(testPath) // Clean up after test

	genFolder(testPath)
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Errorf("Expected folder %s to be created", testPath)
	}
}

func TestAddFileEntries(t *testing.T) {
	testPath := "test_folder"
	os.Mkdir(testPath, os.ModePerm)
	defer os.RemoveAll(testPath) // Clean up after test

	// Create a dummy file
	file, err := os.Create(filepath.Join(testPath, "test_file.txt"))
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	file.Close()

	var entries []FileData
	addFileEntries(testPath, &entries)

	if len(entries) != 1 {
		t.Errorf("Expected 1 file entry, got %d", len(entries))
	}
}

func TestReadMediaInfo(t *testing.T) {
	// This test assumes you have a valid image file with EXIF data
	testFilePath := "test_image.jpg"
	// Create a dummy file for testing
	file, err := os.Create(testFilePath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	file.Close()
	defer os.Remove(testFilePath) // Clean up after test

	info := readMediaInfo(testFilePath)
	if info == "" {
		t.Errorf("Expected EXIF data, got empty string")
	}
}

func TestGroupWorker(t *testing.T) {
	// Create dummy file entries
	entries := []FileData{
		{path: "src1.txt", new_path: "dst1.txt"},
		{path: "src2.txt", new_path: "dst2.txt"},
	}

	// Create dummy files
	for _, entry := range entries {
		file, err := os.Create(entry.path)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		file.Close()
		defer os.Remove(entry.path) // Clean up after test
	}

	groupWorker(entries, 0, len(entries)-1)

	for _, entry := range entries {
		if _, err := os.Stat(entry.new_path); os.IsNotExist(err) {
			t.Errorf("Expected file %s to be created", entry.new_path)
		}
		os.Remove(entry.new_path) // Clean up after test
	}
}

func TestCopy(t *testing.T) {
	src := "src.txt"
	dst := "dst.txt"
	content := []byte("Hello, World!")

	// Create source file
	err := os.WriteFile(src, content, 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}
	defer os.Remove(src) // Clean up after test

	_, err = copy(src, dst)
	if err != nil {
		t.Fatalf("Failed to copy file: %v", err)
	}
	defer os.Remove(dst) // Clean up after test

	// Verify content
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

	// Create source file
	err := os.WriteFile(src, content, 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}
	defer os.Remove(src) // Clean up after test

	_, err = move(src, dst)
	if err != nil {
		t.Fatalf("Failed to move file: %v", err)
	}
	defer os.Remove(dst) // Clean up after test

	// Verify content
	dstContent, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}
	if string(dstContent) != string(content) {
		t.Errorf("Expected content %s, got %s", string(content), string(dstContent))
	}
}
