package main

import (
	"flag"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/tidwall/gjson"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

//type MediaInfo struct {
//	createDate string
//}

// var queue // enhance for concurrent handling the dentries
var dentries = make(map[string][]string)
var entries = make(map[string]os.FileInfo)
var folderformat = "ymd"
var srcPath = ""
var generated = ""

func main() {
	//Reading the image parsing path
	srcPathInput := flag.String("d", ".", "Directory path (absolute path)")

	//Reading the folder creation format
	formatInput := flag.String("f", "ymd", "Folder format (ymd/ym)")

	//Reading the folder creation format
	generatedInput := flag.String("g", "generated", "Generated folder name")

	flag.Parse()
	srcPath = *srcPathInput
	folderformat = *formatInput
	generated = *generatedInput

	println("Working on system path: " + srcPath)
	println("Folder creation format: " + folderformat)

	addFileEntries(srcPath, entries)

	genFolder(srcPath, generated)

	for key, folder_files := range dentries {
		if len(folder_files) > 0 {
			genFolder(srcPath, generated, key)
		}
		for _, fileToMove := range folder_files {
			//Move Files
			//println(path.Join(srcPath, generated, key, filepath.Base(fileToMove)))
			err := os.Rename(fileToMove, path.Join(srcPath, generated, key, filepath.Base(fileToMove)))
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	println(len(entries))
}

func genFolder(paths ...string) {
	var fullpath = path.Join(paths...)
	if _, err := os.Stat(fullpath); os.IsNotExist(err) {
		_ = os.Mkdir(fullpath, os.ModePerm)
		// TODO: handle error
	}
}

func addFileEntries(srcPath string, entries map[string]os.FileInfo) {
	var infoStr string

	files, e := os.ReadDir(srcPath)
	if e != nil {
		panic(e)
	}

	println("Number of files: ", len(files))

	for _, file := range files {
		if file.IsDir() {
			//addFileEntries(, entries)
			println("folder", path.Join(srcPath, file.Name()))
			if file.Name() != generated { //prevent rescan moved files
				addFileEntries(path.Join(srcPath, file.Name()), entries)
			}
		} else {
			var fileInfo, _ = file.Info()
			// println(fileInfo.Name(), fileInfo.Size())
			entries[path.Join(srcPath, file.Name())] = fileInfo

			infoStr = readMediaInfo(path.Join(srcPath, file.Name()))

			if infoStr != "" {
				createTime, _ := time.Parse("2006:01:02 15:04:05", gjson.Get(infoStr, "DateTimeOriginal").String())

				switch folderformat {
				case "ymd":
					addDateEntries(createTime.Format("20060102"), path.Join(srcPath, file.Name()), dentries)
				case "ym":
					addDateEntries(createTime.Format("200601"), path.Join(srcPath, file.Name()), dentries)
				}

				//fmt.Println(t.Format("20060102150405"))
				// addDateEntries(value, dentries)
			}
		}
	}
}

func addDateEntries(dateInput string, fileName string, entries map[string][]string) {
	if _, ok := entries[dateInput]; ok {
		entries[dateInput] = append(entries[dateInput], fileName)
		//do nothing
	} else {
		entries[dateInput] = []string{fileName}
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
	case ".JPG", ".JPEG", ".PNG":
		imgFile, err = os.Open(filePath)
		if err != nil {
			log.Fatal(err.Error())
		}

		metaData, err = exif.Decode(imgFile)
		if err != nil {
			log.Fatal(err.Error())
		}

		jsonByte, err = metaData.MarshalJSON()
		if err != nil {
			log.Fatal(err.Error())
		}

		jsonString = string(jsonByte)

		return jsonString
	default:
		println(ext, "fxxx not supported")
		return ""
	}
	return ""
}
