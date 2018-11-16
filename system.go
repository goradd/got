package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// ExecuteShellCommand executes a shell command in the current working directory and returns its output, if any.
// The result is the combination of stdOut and stdErr
func ExecuteShellCommand(command string) (result string, err error) {
	parts := strings.Split(command, " ")
	if len(parts) == 0 {
		return
	}

	cmd := exec.Command(parts[0], parts[1:]...)

	var out []byte
	out, err = cmd.CombinedOutput()
	result = string(out)
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
	var outText string

	outText, err = ExecuteShellCommand("go list -m -json all")

	if err == nil {
		if outText != "" {
			ret = make (map[string]string)
			// outText is not exactly json. We have to kind of tokenize it
			outText = strings.Replace(outText, "\n}\n{", "\n}****{", -1)	// might not work on windows?
			outs := strings.Split(outText, "****")
			for _, out := range outs {
				out = strings.TrimSpace(out)
				if out != "" && out[0:1] == "{" {
					var v pathType11
					err = json.Unmarshal([]byte(out), &v)
					if err != nil {
						return nil,fmt.Errorf("Error unpacking json from go list command.\n%s\n%s", out, err.Error())
					}
					ret[v.Path] = v.Dir
				}
			}
		}
		return
	} else {
		// We don't have module support, so everything flows from top level locations
		if outText, err = ExecuteShellCommand("go list -find -json all"); err != nil {
			return nil,fmt.Errorf("Error executing shell command %s, %s", outText, err.Error())
		}

		root := runtime.GOROOT()	// we are going to remove built in packages
		if outText != "" {
			ret = make (map[string]string)
			// outText is not exactly json. We have to kind of tokenize it
			outText = strings.Replace(outText, "\n}\n{", "\n}****{", -1)	// might not work on windows?
			outs := strings.Split(outText, "****")
			for _, out := range outs {
				out = strings.TrimSpace(out)
				if out != "" && out[0:1] == "{" {
					var v pathType10
					err = json.Unmarshal([]byte(out), &v)
					if err != nil {
						return nil,fmt.Errorf("Error unpacking json from go list command.\n%s\n%s", out, err.Error())
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

