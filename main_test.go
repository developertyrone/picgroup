package main

import (
	"os"
	"testing"
)

func BenchmarkFileEntries(b *testing.B) {
	var entries = make(map[string]os.FileInfo)

	addFileEntries("/Users/superkufu/Desktop/testimage2", entries)
}
