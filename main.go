package main

import (
	"flag"
	"fmt"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/tidwall/gjson"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type FileData struct {
	path     string
	info     os.FileInfo
	new_path string
}

// var queue // enhance for concurrent handling the dentries
var date_folders = make(map[string]bool, 0)
var fentries = make([]FileData, 0)
var folder_format = "ymd"
var src_path = ""
var generated = ""
var copy_mode = ""
var verbose_mode = ""
var group_mode = ""

func main() {

	//Reading the image parsing path
	src_pathInput := flag.String("d", ".", "Directory path (absolute path)")

	//Reading the folder creation format
	formatInput := flag.String("f", "ymd", "Folder format (ymd/ym)")

	//Reading the folder creation format
	generatedInput := flag.String("t", "generated", "Generated folder name")

	//Group mode
	group_modeInput := flag.String("g", "move", "Grouping mode (move/copy)")

	//Reading the folder creation format
	copy_modeInput := flag.String("m", "seq", "File copy mode (seq/con)")

	//Reading the folder creation format
	verboseInput := flag.String("v", "0", "Verbose mode (0/1)")

	flag.Parse()

	parseInput(*src_pathInput, *formatInput, *generatedInput, *copy_modeInput, *verboseInput, *group_modeInput)

	if verbose_mode == "1" {
		defer timeTrack(time.Now(), "process")
	}

	addFileEntries(src_path, &fentries)

	orgnizeFiles()

	clear()
}

func clear() {
	date_folders = make(map[string]bool, 0)
	fentries = make([]FileData, 0)
}

func parseInput(src, format, gen, copy, verbose, group string) {

	src_path = src
	folder_format = format
	generated = gen
	copy_mode = copy
	verbose_mode = verbose
	group_mode = group

	if src_path == "." {
		println("Please define a path with -d and -h for help ")
		os.Exit(0)
	}

	if verbose_mode == "1" {
		println("Working on system path: " + src_path)
		println("Folder creation format: " + folder_format)
		println("Copy mode: " + copy_mode)
	}

}

func orgnizeFiles() {

	if verbose_mode == "2" {
		println("Generated folders:  ", len(date_folders))
		println("All processed files entries: ", len(fentries))
	}

	if len(date_folders) > 0 {

		genFolder(src_path, generated)

		//create folder place holders
		for i, _ := range date_folders {
			genFolder(src_path, generated, i)
		}

		switch copy_mode {
		case "seq":
			groupWorker(fentries, 0, len(fentries)-1)
		case "con":
			num_of_worker := MaxParallelism()
			if verbose_mode == "1" {
				println("Number of workers: ", num_of_worker)
			}

			//Wait Group implementation
			var wg sync.WaitGroup
			//To enhance the number of go routines[TODO]
			var no_of_seg = len(fentries) / num_of_worker // 229 = 22

			for i := 0; i <= len(fentries)/num_of_worker; i++ {
				wg.Add(1)
				go func(routine int, start int, end int) {
					if end > len(fentries)-1 {
						end = len(fentries) - 1
					}
					defer wg.Done()
					if verbose_mode == "1" {
						println("start copy routine ", routine, "from", start, "to", end)
					}
					groupWorker(fentries, start, end)
				}(i, i*no_of_seg, (i+1)*no_of_seg-1)
			}
			wg.Wait()
		}

	}

	if verbose_mode == "1" {
		println(len(fentries))
	}

}

func genFolder(paths ...string) {
	var fullpath = path.Join(paths...)
	if _, err := os.Stat(fullpath); os.IsNotExist(err) {
		_ = os.Mkdir(fullpath, os.ModePerm)
		// TODO: handle error
	}
}

func addFileEntries(from_path string, entries *[]FileData) {
	var infoStr string

	files, e := os.ReadDir(from_path)
	if e != nil {
		panic(e)
	}

	if verbose_mode == "1" {
		println(from_path, "has ", len(files), " files")
	}

	for _, file := range files {
		if file.IsDir() {
			//addFileEntries(, entries)
			//println("folder", path.Join(from_path, file.Name()))
			if file.Name() != generated && !strings.HasPrefix(file.Name(), ".") && !strings.HasPrefix(file.Name(), "@") { //prevent rescan moved files
				addFileEntries(path.Join(from_path, file.Name()), entries)
			}
		} else {
			var fileInfo, _ = file.Info()

			//Check file original create date, if not supported, ignore the file
			infoStr = readMediaInfo(path.Join(from_path, file.Name()))

			if infoStr != "" {
				createTime, _ := time.Parse("2006:01:02 15:04:05", gjson.Get(infoStr, "DateTimeOriginal").String())
				var curPath = path.Join(from_path, file.Name())

				switch folder_format {
				case "ymd":
					*entries = append(*entries,
						FileData{
							path:     curPath,
							info:     fileInfo,
							new_path: path.Join(src_path, generated, createTime.Format("20060102"), filepath.Base(curPath))},
					)
					//date_folders = append(date_folders, path.Join(src_path, generated, createTime.Format("20060102")))
					//date_folders[path.Join(src_path, generated, createTime.Format("20060102"))] = true
					date_folders[createTime.Format("20060102")] = true
					//addDateEntries(createTime.Format("20060102"), path.Join(src_path, file.Name()), dentries)
				case "ym":
					*entries = append(*entries,
						FileData{
							path:     curPath,
							info:     fileInfo,
							new_path: path.Join(src_path, generated, createTime.Format("200601"), filepath.Base(curPath))},
					)
					//date_folders[path.Join(src_path, generated, createTime.Format("200601"))] = true
					date_folders[createTime.Format("200601")] = true
					//addDateEntries(createTime.Format("200601"), path.Join(src_path, file.Name()), dentries)
				}

			}

		}
	}
}

func readMediaInfo(filePath string) string {
	var err error
	var imgFile *os.File
	var metaData *exif.Exif
	var jsonByte []byte
	var jsonString string

	var ext = strings.ToUpper(filepath.Ext(filePath))

	switch ext {
	case ".JPG", ".JPEG", ".PNG", ".ARW":
		imgFile, err = os.Open(filePath)
		if err != nil {
			//log.Fatal(err.Error())
			log.Println(err.Error())
			return ""
		}

		metaData, err = exif.Decode(imgFile)
		if err != nil {
			//log.Fatal(err.Error())
			log.Println(err.Error())
			return ""
		}

		jsonByte, err = metaData.MarshalJSON()
		if err != nil {
			//log.Fatal(err.Error())
			log.Println(err.Error())
			return ""
		}

		jsonString = string(jsonByte)

		return jsonString
	default:
		if verbose_mode == "1" {
			println(ext, "fxxx not supported")
		}
		return ""
	}
	return ""
}

func groupWorker(fileEntries []FileData, start int, end int) {
	for index := start; index <= end; index++ {
		if index < len(fileEntries) {
			if verbose_mode == "1" {
				println(group_mode, " pair ", index, fileEntries[index].path, fileEntries[index].new_path)
			}
			switch group_mode {
			case "copy":
				copy(fileEntries[index].path, fileEntries[index].new_path)
			case "move":
				move(fileEntries[index].path, fileEntries[index].new_path)
			}

		}
	}
}

func copy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
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

func move(src, dst string) (int64, error) {
	err := os.Rename(src, dst)
	if err != nil {
		return 0, err
	}
	return 1, nil
}

func MaxParallelism() int {
	maxProcs := runtime.GOMAXPROCS(0)
	numCPU := runtime.NumCPU()
	if maxProcs < numCPU {
		return maxProcs
	}
	return numCPU
}

func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("%s took %s", name, elapsed)
}
