package got

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/goradd/gofile/pkg/sys"
)

type namedBlockEntry struct {
	text       string
	paramCount int
	ref        locationRef
}

var modules map[string]string
var includePaths []string
var includeNamedBlocks = make(map[string]namedBlockEntry)

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
				_, _ = fmt.Fprintf(os.Stderr, "Include path %s: %s", p, err.Error())
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
			_, _ = fmt.Fprintf(os.Stderr, "Could not use the current directory as the output directory: %s", err.Error())
			return 1
		}
	}
	outDir = getRealPath(outDir)

	dstInfo, err := os.Stat(outDir)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "The output directory %s does not exist. Create the output directory and run it again.", outDir)
		return 1
	}

	if !dstInfo.Mode().IsDir() {
		_, _ = fmt.Fprintf(os.Stderr, "The output directory specified is not a directory")
		return 1
	}

	if typ != "" {
		files, _ = filepath.Glob(inputDirectory + "*." + typ)
	}

	asts, err := prepIncludeFiles(includeFiles)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, err.Error())
		return 1
	}

	// TODO: parallel multi-file processing with go routines
	var files2 []string
	for _, file := range files {
		newPath := outfilePath(file, outDir)
		// duplicate the named blocks from the include files before passing them to individual files
		namedBlocks := make(map[string]namedBlockEntry)
		for k, v := range includeNamedBlocks {
			namedBlocks[k] = v
		}

		// Default named block values
		file, _ = filepath.Abs(file)
		root := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		for {
			ext := filepath.Ext(root)
			if ext == "" {
				break
			}
			root = strings.TrimSuffix(root, ext)
		}

		namedBlocks["templatePath"] = namedBlockEntry{file, 0, locationRef{}}
		namedBlocks["templateName"] = namedBlockEntry{filepath.Base(file), 0, locationRef{}}
		namedBlocks["templateRoot"] = namedBlockEntry{root, 0, locationRef{}}
		namedBlocks["templateParent"] = namedBlockEntry{filepath.Base(filepath.Dir(file)), 0, locationRef{}}

		newPath, _ = filepath.Abs(newPath)
		root = strings.TrimSuffix(filepath.Base(newPath), filepath.Ext(newPath))
		for {
			ext := filepath.Ext(root)
			if ext == "" {
				break
			}
			root = strings.TrimSuffix(root, ext)
		}

		namedBlocks["outPath"] = namedBlockEntry{newPath, 0, locationRef{}}
		namedBlocks["outName"] = namedBlockEntry{filepath.Base(newPath), 0, locationRef{}}
		namedBlocks["outRoot"] = namedBlockEntry{root, 0, locationRef{}}
		namedBlocks["outParent"] = namedBlockEntry{filepath.Base(filepath.Dir(newPath)), 0, locationRef{}}

		var a astType
		a, err = buildAst(file, namedBlocks)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, err.Error())
			return 1
		}

		var asts2 []astType
		asts2 = append(asts2, asts...)
		asts2 = append(asts2, a)

		err = outputAsts(newPath, asts2...)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, err.Error())
			return 1
		}

		files2 = append(files2, newPath)
	}

	// Since go typically does io asynchronously, we run our second stage after some pause to let the writes finish
	for _, file := range files2 {
		if n := postProcess(file, runImports); n > 0 {
			return n
		}
	}
	return 0
}

func prepIncludeFiles(includes []string) (asts []astType, err error) {
	for _, f := range includes {
		var a astType
		a, err = buildAst(f, includeNamedBlocks)
		if err == nil {
			asts = append(asts, a)
		} else {
			break
		}
	}
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

func postProcess(file string, runImports bool) int {
	curDir, _ := os.Getwd()
	dir := filepath.Dir(file)
	_ = os.Chdir(dir) // run it from the file's directory to pick up the correct go.mod file if there is one
	if runImports {
		_, err := sys.ExecuteShellCommand("goimports -w " + filepath.Base(file))
		if err != nil {
			if e, ok := err.(*exec.Error); ok {
				_, _ = fmt.Fprintln(os.Stderr, "error running goimports on file "+file+": "+e.Error()) // perhaps goimports is not installed?
				return 1
			} else if err2, ok2 := err.(*exec.ExitError); ok2 {
				// Likely a syntax error in the resulting file
				_, _ = fmt.Fprintln(os.Stderr, string(err2.Stderr))
				return 1
			}
		}
	}
	_ = os.Chdir(curDir)
	return 0
}
