package main

import (
	"flag"
	"fmt"
	"github.com/goradd/gofile/pkg/sys"
	"github.com/goradd/got/got"
	"os/exec"

	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var args string // A neat little trick to directly test the main function. If we are testing, this will get set.
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
	flag.StringVar(&includes, "I", "", "The list of directories to look in to find template include files.")
	flag.StringVar(&inputDirectory, "d", "", "The directory to search for files if using the -t directive. Otherwise the current directory will be searched.")
	if args == "" {
		flag.Parse() // regular run of program
	} else {
		// test run
		flag.CommandLine.Parse(strings.Split(args, " "))
	}
	files := flag.Args()

	var err error
	if modules, err = sys.ModulePaths(); err != nil {
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
		return
	}

	got.IncludePaths = []string{}
	got.IncludeFiles = []string{}
	if includes != "" {
		for includes != "" {
			var cur string
			if offset := strings.IndexAny(includes, ":;"); offset != -1 {
				cur = includes[:offset]
				includes = includes[offset+1:]
			} else {
				cur = includes
				includes = ""
			}
			p := getRealPath(cur)
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

	if outDir == "" {
		if outDir, err = os.Getwd(); err != nil {
			panic("Could not use the current directory as the output directory.")
		}
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

	var files2 []string
	for _, file := range files {
		s := processFile(file)
		if s != "" {
			f := writeFile(s, file, outDir)
			files2 = append(files2, f)
		}
	}

	// Since go typically does io asynchronously, we run our second stage after some pause to let the writes finish
	for _, file := range files2 {
		postProcess(file, runImports)
	}
}

func getRealPath(path string) string {
	newPath, err := sys.GetModulePath(path, modules)
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
		s = "//** This file was code generated by got. DO NOT EDIT. ***\n\n\n" + s
	}
	return s
}

func writeFile(s string, file string, outDir string) string {

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

	file = filepath.Join(dir, file)

	err := ioutil.WriteFile(file, []byte(s), os.ModePerm)
	if err != nil {
		panic("Could not write file " + file + ": " + err.Error())
	}
	return file
}

func postProcess(file string, runImports bool) {
	curDir, _ := os.Getwd()
	dir := filepath.Dir(file)
	_ = os.Chdir(dir) // run it from the file's directory to pick up the correct go.mod file if there is one
	if runImports {
		_, err := sys.ExecuteShellCommand("goimports -w " + filepath.Base(file))
		if err != nil {
			if e, ok := err.(*exec.Error); ok {
				panic("error running goimports on file " + file + ": " + e.Error()) // perhaps goimports is not installed?
			} else if e, ok := err.(*exec.ExitError); ok {
				// Likely a syntax error in the resulting file
				log.Print(string(e.Stderr))
			}
		}
	} else {
		_, err := sys.ExecuteShellCommand("go fmt " + file) // at least format it if we are not going to run imports on it
		if err != nil {
			if e, ok := err.(*exec.Error); ok {
				panic("error running goimports on file " + file + ": " + e.Error()) // perhaps goimports is not installed?
			} else if e, ok := err.(*exec.ExitError); ok {
				// Likely a syntax error in the resulting file
				log.Print(string(e.Stderr))
			}
		}
	}
	_ = os.Chdir(curDir)
}

//Process a string that is a got template, and return the go code
func ProcessString(input string, fileName string) string {
	l := got.Lex(input, fileName)

	s := got.Parse(l)

	return s
}
