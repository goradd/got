package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/spekary/got/got"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
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
		i := strings.Split(includes, ";")
		for _, i2 := range i {
			path := getRealPath(i2)
			if fi, err := os.Stat(path); err != nil {
				fmt.Println("Include path " + path + " does not exist.")
			} else if fi.IsDir() {
				got.IncludePaths = append(got.IncludePaths, path)
			} else {
				got.IncludeFiles = append(got.IncludeFiles, path)
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
	path = filepath.FromSlash(path)

	for modPath,dir := range modules {
		if len(modPath) <= len(path) && path[:len(modPath)] == modPath {	// if the path starts with a module path, replace it with the actual directory
			path = filepath.Join(dir, path[len(modPath):])
			break
		}
	}

	var err error
	path, err = filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	return path
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

	if dir != "/" {
		dir = dir + "/"
	}
	file = dir + file

	ioutil.WriteFile(file, []byte(s), os.ModePerm)

	if runImports {
		ExecuteShellCommand("goimports -w " + file)
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

// ExecuteShellCommand executes a shell command in the current working directory and returns its output, if any.
func ExecuteShellCommand(command string) (stdOutText string, err error) {
	parts := strings.Split(command, " ")
	if len(parts) == 0 {
		return
	}

	cmd := exec.Command(parts[0], parts[1:]...)

	var stdOut []byte
	stdOut, err = cmd.Output()
	stdOutText = string(stdOut)
	return
}

type pathType11 struct {
	Path string
	Dir string
}

type pathType10 struct {
	ImportPath string
	Dir string
}


// ModulePaths returns a listing of the paths of all the modules included in the build, keyed by module name, as if the build was run from the
// current working directory. Note that Go's module support can change a build based on the go.mod file found, which is dependent
// on the current working directory.
// If we are building without module support, it will return ALL imported packages that are not standard go packages,
// not just the module paths.
func ModulePaths() (ret map[string]string, err error) {
	var outText string

	if !GoVersionGreaterThan("1.10") {
		if outText, err = ExecuteShellCommand("go list -m -json all"); err != nil {
			return
		}

		if outText != "" {
			ret = make (map[string]string)
			// outText is not exactly json. We have to kind of tokenize it
			outs := strings.SplitAfter(outText, "}")
			for _, out := range outs {
				out = strings.TrimSpace(out)
				if out != "" && out[0:1] == "{" {
					var v pathType11
					err = json.Unmarshal([]byte(out), &v)
					if err != nil {
						return
					}
					ret[v.Path] = v.Dir
				}
			}
		}
		return
	} else {
		// We don't have module support
		if outText, err = ExecuteShellCommand("go list -find -json all"); err != nil {
			return
		}

		root := runtime.GOROOT()	// we are going to remove built in packages
		if outText != "" {
			ret = make (map[string]string)
			// outText is not exactly json. We have to kind of tokenize it
			outs := strings.SplitAfter(outText, "}\n") // might not work on windows?
			for _, out := range outs {
				out = strings.TrimSpace(out)
				if out != "" && out[0:1] == "{" {
					var v pathType10
					err = json.Unmarshal([]byte(out), &v)
					if err != nil {
						return
					}
					if len(root) <= len(v.Dir) && v.Dir[:len(root)] != root {
						ret[v.ImportPath] = v.Dir
					}
				}
			}
		}
		return
	}
}

// Returns true if the Go version is greater than that given. It will check how ever deep you ask.
// So 1.10.3 is not greater than 1.10, but it is greater than 1.10.1.
func GoVersionGreaterThan(ver string) bool {
	realVers := strings.Split(runtime.Version(), ".")
	realVers[0] = realVers[0][2:]

	vers := strings.Split(ver, ".")

	for i,n := range vers {
		v,_ := strconv.Atoi(n)
		v2, _ := strconv.Atoi(realVers[i])
		if v < v2 {
			return true
		}
	}
	return false
}

