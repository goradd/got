package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/goradd/got/internal/got"
)

var args string // A neat little trick to directly test the main function. If we are testing, this will get set.

func main() {
	var outDir string
	var typ string
	var runImports bool
	var includes string
	var inputDirectory string

	if len(os.Args[1:]) == 0 || args == "testEmpty" {
		fmt.Println("got processes got template files, turning them into go code to use in your application.")
		fmt.Println("Usage: got [-o outDir] [-t fileType] [-i] [-I includeDirs] file1 [file2 ...] ")
		fmt.Println("-o: send processed files to the given directory. Otherwise sends to the same directory that the template is in.")
		fmt.Println("-t: process all files with this suffix in the current directory. Otherwise, specify specific files at the end.")
		fmt.Println("-i: run goimports on the result files to automatically fix up the import statement and format the file. You will need goimports installed.")
		fmt.Println("-I: the list of directories to search for include files, or files to prepend before every processed file. Files are searched in the order given, and first one found will be used.")
		fmt.Println("-d: The directory to search for files if using the -t directive.")
		return
	}

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

	if ret := got.Run(outDir, typ, runImports, includes, inputDirectory, files); ret != 0 {
		os.Exit(ret)
	}
}
