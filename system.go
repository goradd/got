package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// ExecuteShellCommand executes a shell command in the current working directory and returns its output, if any.
// The result is stdOut. If you get an error, you can cast err to (*exec.ExitError) and read the stdErr member to see
// the error message that was generated.
func ExecuteShellCommand(command string) (result []byte, err error) {
	parts := strings.Split(command, " ")
	if len(parts) == 0 {
		return
	}

	cmd := exec.Command(parts[0], parts[1:]...)

	result, err = cmd.Output()
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
//
// If we are building without module support, it will return only the top paths to packages, since everything in this
// situation will be relative to GOPATH.
func ModulePaths() (ret map[string]string, err error) {
	var outText []byte

	outText, err = ExecuteShellCommand("go list -m -json all")

	if err == nil {
		if outText != nil && len(outText) > 0 {
			ret = make (map[string]string)
			dec := json.NewDecoder(bytes.NewReader(outText))
			for {
				var v pathType11
				if err := dec.Decode(&v); err != nil {
					if err == io.EOF {
						break
					}
					return nil,fmt.Errorf("Error unpacking json from go list command.\n%s\n%s", string(outText), err.Error())
				}
				ret[v.Path] = v.Dir
			}
		}
		return
	} else {
		// We don't have module support, so everything flows from top level locations
		if outText, err = ExecuteShellCommand("go list -find -json all"); err != nil {
			return nil,fmt.Errorf("Error executing shell command %s, %s", outText, err.Error())
		}

		root := runtime.GOROOT()	// we are going to remove built in packages
		if outText != nil && len(outText) > 0 {
			ret = make (map[string]string)
			dec := json.NewDecoder(bytes.NewReader(outText))
			for {
				var v pathType10
				if err := dec.Decode(&v); err != nil {
					if err == io.EOF {
						break
					}
					return nil,fmt.Errorf("Error unpacking json from go list command.\n%s\n%s", string(outText), err.Error())
				}
				if len(root) <= len(v.Dir) && v.Dir[:len(root)] != root { // exclude built-in packages
					// truncate the path up to the top level. We have to try to preserve the same format we were given.
					pathItems := strings.Split(v.ImportPath, "/")
					p := path.Join(pathItems[1:]...)
					dir := v.Dir[:len(v.Dir) - len(p) - 1]
					ret[pathItems[0]] = dir
				}
			}
		}
		return
	}
}

// GetModulePath compares the given path with the list of modules and if the path begins with a module name, it will
// substitute the absolute path for the module name. It will clean the path given as well.
// modules is the output from ModulePaths.
func GetModulePath(path string, modules map[string]string) (newPath string, err error) {
	for modPath,dir := range modules {
		if len(modPath) <= len(path) && path[:len(modPath)] == modPath {	// if the path starts with a module path, replace it with the actual directory
			path = filepath.Join(dir, path[len(modPath):])
			break
		}
	}

	path = filepath.FromSlash(path)

	newPath, err = filepath.Abs(path)
	return
}

