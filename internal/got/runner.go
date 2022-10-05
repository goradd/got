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

	if modules, err = sys.ModulePaths(); err != nil {
		return err
	}

	if inputDirectory != "" {
		inputDirectory = getRealPath(inputDirectory)
		if inputDirectory[len(inputDirectory)-1] != filepath.Separator {
			inputDirectory += string(filepath.Separator)
		}
	}

	if typ != "" {
		files, _ = filepath.Glob(inputDirectory + "*." + typ)
	}

	var cwd string
	cwd, err = os.Getwd()
	if err != nil {
		return fmt.Errorf("could not get the current directory: %s", err.Error())
	}
	for _, file := range files {
		f := file
		fmt.Printf("Processing %s\n", f)
		f = filepath.FromSlash(f)
		dir, _ := filepath.Split(f)
		if dir != "" {
			if err = os.Chdir(dir); err != nil {
				return fmt.Errorf("could not change to directory %s:%s", dir, err.Error())
				return err
			}
		}

		var includeFiles []string
		includeFiles, includePaths, err = processIncludeString(includes)
		if err != nil {
			return err
		}

		if inputDirectory == "" || dir == "" {
			includePaths = append(includePaths, cwd)
		} else {
			includePaths = append(includePaths, dir)
		}

		outDir2 := outDir
		if outDir2 == "" {
			outDir2 = dir
			if outDir2 == "" {
				outDir2 = cwd
			}
		}
		outDir2 = getRealPath(outDir2)

		dstInfo, err2 := os.Stat(outDir2)
		if err2 != nil {
			return fmt.Errorf("the output directory %s does not exist. Create the output directory and run it again", outDir2)
		}
		if !dstInfo.Mode().IsDir() {
			return fmt.Errorf("the output directory specified is not a directory")
		}

		asts, err3 := prepIncludeFiles(includeFiles)
		if err3 != nil {
			return err3
		}

		err = processFile(f, outDir2, asts, runImports)

		if dir != "" {
			if err2 := os.Chdir(cwd); err2 != nil {
				return fmt.Errorf("could not change to cwd %s:%s", cwd, err2.Error())
			}
		}
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
	return postProcess(newPath, runImports)
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
	newPath = filepath.FromSlash(newPath)
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
		_, err = sys.ExecuteShellCommand("goimports -w " + file)
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
