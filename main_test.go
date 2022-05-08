package main

import "testing"

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
