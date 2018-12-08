package main

import (
	"flag"
	"fmt"
	"github.com/spekary/got/got"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var args string	// A neat little trick to directly test the main function. If we are testing, this will get set.
var modules map[string]string

func main() {
	var outDir string
	var typ string
	var runImports bool
	var includes string
	var inputDirectory string

	flag.StringVar(&outDir, "o", "", "Output directory")
	flag.StringVar(&typ, "t", "", "Will process all files with this suffix in current directory, or the directory given by the -d directive.")
	flag.BoolVar(&runImports, "i", false, "Run goimports on the file to automatically add your imports to the file. You will need to install goimports to do this.")
	flag.StringVar(&includes, "I", "", "The list of directories to look in to find template include files. Separate with semicolons.")
	flag.StringVar(&inputDirectory, "d", "", "The directory to search for files if using the -t directive. Otherwise the current directory will be searched.")
	if args == "" {
		flag.Parse() // regular run of program
	} else {
		// test run
		flag.CommandLine.Parse(strings.Split(args, " "))
	}
	files := flag.Args()

	var err error
	if modules,err = ModulePaths(); err != nil {
		panic(err)
	}

	if len(os.Args[1:]) == 0 {
		fmt.Println("got processes got template files, turning them into go code to use in your application.")
		fmt.Println("Usage: got [-o outDir] [-t fileType] [-i] [-I includeDirs] file1 [file2 ...] ")
		fmt.Println("-o: send processed files to the given directory. Otherwise sends to the same directory that the template is in.")
		fmt.Println("-t: process all files with this suffix in the current directory. Otherwise, specify specific files at the end.")
		fmt.Println("-i: run goimports on the result files to automatically fix up the import statement and format the file. You will need goimports installed.")
		fmt.Println("-I: the list of directories to search for include files, or files to prepend before every processed file. Files are searched in the order given, and first one found will be used.")
		fmt.Println("-d: The directory to search for files if using the -t directive.")
	}

	got.IncludePaths = []string{}
	got.IncludeFiles = []string{}
	if includes != "" {
		i := filepath.SplitList(includes)
		for _, i2 := range i {
			p := getRealPath(i2)
			if fi, err := os.Stat(p); err != nil {
				fmt.Println("Include path " + p + " does not exist.")
			} else if fi.IsDir() {
				got.IncludePaths = append(got.IncludePaths, p)
			} else {
				got.IncludeFiles = append(got.IncludeFiles, p)
			}
		}
	}

	if inputDirectory != "" {
		inputDirectory = getRealPath(inputDirectory)
		if inputDirectory[len(inputDirectory)-1] != filepath.Separator {
			inputDirectory += string(filepath.Separator)
		}
	}

	if inputDirectory == "" {
		got.IncludePaths = append(got.IncludePaths, getRealPath("."))
	} else {
		got.IncludePaths = append(got.IncludePaths, inputDirectory)
	}

	outDir = getRealPath(outDir)

	dstInfo, err := os.Stat(outDir)
	if err != nil {
		panic(fmt.Sprintf("The output directory %s does not exist. Create the output directory and run it again.", outDir))
	}

	if !dstInfo.Mode().IsDir() {
		panic("The output directory specified is not a directory")
	}

	//var err error

	if typ != "" {
		files, _ = filepath.Glob(inputDirectory + "*." + typ)
	}

	for _, file := range files {
		s := processFile(file)
		if s != "" {
			writeFile(s, file, outDir, runImports)
		}
	}
}

func getRealPath(path string) string {
	newPath, err := GetModulePath(path, modules)
	if err != nil {
		log.Fatal(err)
	}
	return newPath
}


func processFile(file string) string {
	var s string

	for _, f := range got.IncludeFiles {
		buf, err := ioutil.ReadFile(f)
		if err != nil {
			fmt.Println(err)
			return ""
		}
		s += string(buf[:])
	}

	buf, err := ioutil.ReadFile(file)

	if err != nil {
		fmt.Println(err)
		return ""
	}
	s += string(buf[:])

	/*
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("Recovered ", r)
			}
		}()*/

	s = ProcessString(s, file)
	if s != "" {
		s = "//** This file was code generated by got. ***\n\n\n" + s
	}
	return s
}

func writeFile(s string, file string, outDir string, runImports bool) {

	dir := filepath.Dir(file)
	dir, _ = filepath.Abs(dir)
	file = filepath.Base(file)

	i := strings.LastIndex(file, ".")

	if i < 0 {
		file = file + ".go"
	} else {
		file = file[:i] + ".go"
	}

	if outDir != "" {
		dir = outDir
	}

	file = filepath.Join(dir,file)

	ioutil.WriteFile(file, []byte(s), os.ModePerm)

	if runImports {
		_,err := ExecuteShellCommand("goimports -w " + file)
		if err != nil {
			panic("error running goimports: " + err.Error())	// perhaps goimports is not installed?
		}
	} else {
		ExecuteShellCommand("go fmt " + file) // at least format it if we are not going to run imports on it
	}
}

//Process a string that is a got template, and return the go code
func ProcessString(input string, fileName string) string {
	l := got.Lex(input, fileName)

	s := got.Parse(l)

	return s
}

