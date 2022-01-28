package got

import (
	"fmt"
	"github.com/goradd/gofile/pkg/sys"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var modules map[string]string
var includePaths []string
var includeNamedBlocks map[string]namedBlockEntry

func Run(outDir string,
	typ string,
	runImports bool,
	includes string,
	inputDirectory string,
	files []string) int {

	var includeFiles []string

	{
		var err error
		if modules, err = sys.ModulePaths(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, err.Error())
			return 1
		}
	}

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
				_,_ = fmt.Fprintf(os.Stderr, "Include path %s: %s", p, err.Error())
				return 1
			} else if fi.IsDir() {
				includePaths = append(includePaths, p)
			} else {
				includeFiles = append(includeFiles, p)
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
		includePaths = append(includePaths, getRealPath("."))
	} else {
		includePaths = append(includePaths, inputDirectory)
	}

	if outDir == "" {
		var err error
		if outDir, err = os.Getwd(); err != nil {
			_,_ = fmt.Fprintf(os.Stderr, "Could not use the current directory as the output directory: %s",  err.Error())
			return 1
		}
	}
	outDir = getRealPath(outDir)

	dstInfo, err := os.Stat(outDir)
	if err != nil {
		_,_ = fmt.Fprintf(os.Stderr, "The output directory %s does not exist. Create the output directory and run it again.", outDir)
		return 1
	}

	if !dstInfo.Mode().IsDir() {
		_,_ = fmt.Fprintf(os.Stderr, "The output directory specified is not a directory")
		return 1
	}

	if typ != "" {
		files, _ = filepath.Glob(inputDirectory + "*." + typ)
	}

	asts,err := prepIncludeFiles(includeFiles)
	if err != nil {
		_,_ = fmt.Fprintf(os.Stderr, err.Error())
		return 1
	}

	var files2 []string
	for _, file := range files {
		newPath := outfilePath(file, outDir)
		// duplicate the named blocks from the include files in case the previous file added to them
		useNamedBlocks(includeNamedBlocks)
		a,err := buildAst(file)
		if err != nil {
			_,_ = fmt.Fprintf(os.Stderr, err.Error())
			return 1
		}

		var asts2 []astType
		asts2 = append(asts2, asts...)
		asts2 = append(asts2, a)

		outputAsts(newPath, asts2...)
		files2 = append(files2, newPath)
	}

	// Since go typically does io asynchronously, we run our second stage after some pause to let the writes finish
	for _, file := range files2 {
		postProcess(file, runImports)
	}
	return 0
}

func prepIncludeFiles(includes []string) (asts []astType, err error) {
	for _, f := range includes {
		var a astType
		a,err = buildAst(f)
		if err == nil {
			asts = append(asts, a)
		} else {
			break
		}
	}
	includeNamedBlocks = getNamedBlocks()
	return
}

func getRealPath(path string) string {
	newPath, err := sys.GetModulePath(path, modules)
	if err != nil {
		log.Fatal(err)
	}
	return newPath
}


func outfilePath(file string, outDir string) string {
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
				_,_ = fmt.Fprintln(os.Stderr, "error running goimports on file " + file + ": " + e.Error()) // perhaps goimports is not installed?
				os.Exit(1)
			} else if err2, ok2 := err.(*exec.ExitError); ok2 {
				// Likely a syntax error in the resulting file
				_,_ = fmt.Fprintln(os.Stderr, err2.Stderr)
				os.Exit(1)
			}
		}
	} else {
		_, err := sys.ExecuteShellCommand("go fmt " + file) // at least format it if we are not going to run imports on it
		if err != nil {
			if e, ok := err.(*exec.Error); ok {
				_,_ = fmt.Fprintln(os.Stderr, "error running go fmt on file " + file + ": " + e.Error()) // perhaps goimports is not installed?
				os.Exit(1)
			} else if e2, ok2 := err.(*exec.ExitError); ok2 {
				// Likely a syntax error in the resulting file
				log.Print(string(e2.Stderr))
			}
		}
	}
	_ = os.Chdir(curDir)
}
