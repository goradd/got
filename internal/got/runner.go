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

// Run processes the given GoT files with the given options.
// It writes to the output files while processing, and returns an error if found.
func Run(outDir string,
	typ string,
	runImports bool,
	includes string,
	inputDirectory string,
	files []string) (err error) {

	var includeFiles []string

	if modules, err = sys.ModulePaths(); err != nil {
		return err
	}

	includeFiles, includePaths, err = processIncludeString(includes)
	if err != nil {
		return err
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
		if outDir, err = os.Getwd(); err != nil {
			return fmt.Errorf("could not use the current directory as the output directory: %s", err.Error())
		}
	}
	outDir = getRealPath(outDir)

	dstInfo, err := os.Stat(outDir)
	if err != nil {
		return fmt.Errorf("the output directory %s does not exist. Create the output directory and run it again", outDir)
	}

	if !dstInfo.Mode().IsDir() {
		return fmt.Errorf("the output directory specified is not a directory")
	}

	if typ != "" {
		files, _ = filepath.Glob(inputDirectory + "*." + typ)
	}

	asts, err2 := prepIncludeFiles(includeFiles)
	if err2 != nil {
		return err2
	}

	for _, file := range files {
		err = processFile(file, outDir, asts, runImports)
		if err != nil {
			return err
		}
	}

	return
}

func processFile(file, outDir string, asts []astType, runImports bool) error {
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

	a, err := buildAst(file, namedBlocks)
	if err != nil {
		return err
	}

	var asts2 []astType
	asts2 = append(asts2, asts...)
	asts2 = append(asts2, a)

	err = outputAsts(newPath, asts2...)
	if err != nil {
		return err
	}
	return postProcess(file, runImports)
}

func processIncludeString(includes string) (includeFiles []string, includePaths []string, err error) {
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
		if fi, err2 := os.Stat(p); err2 != nil {
			err = fmt.Errorf("include path %s: %s", p, err2.Error())
			return
		} else if fi.IsDir() {
			includePaths = append(includePaths, p)
		} else {
			includeFiles = append(includeFiles, p)
		}
	}
	return
}

func prepIncludeFiles(includeFiles []string) (asts []astType, err error) {
	for _, f := range includeFiles {
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

func postProcess(file string, runImports bool) (err error) {
	if runImports {
		curDir, _ := os.Getwd()
		dir := filepath.Dir(file)
		_ = os.Chdir(dir) // run it from the file's directory to pick up the correct go.mod file if there is one
		_, err = sys.ExecuteShellCommand("goimports -w " + filepath.Base(file))
		_ = os.Chdir(curDir)
		if err != nil {
			if e, ok := err.(*exec.Error); ok {
				return fmt.Errorf("error running goimports on file %s: %s", file, e.Error())
			} else if err2, ok2 := err.(*exec.ExitError); ok2 {
				// Likely a syntax error in the resulting file
				return fmt.Errorf("%s", err2.Stderr)
			}
		}
	}
	return nil
}
