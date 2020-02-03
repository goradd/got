package main

import (
	"github.com/goradd/gofile/pkg/sys"
	"testing"
	"path/filepath"
	"os"
	"io/ioutil"
	"bytes"
)

// This file runs the tests found in the test directory. It is set up so that code coverage can be checked as well.
// Doing this is a little tricky, since got generates go code that then gets compiled and run again. Each part of the
// process may generate errors. We test the process from end to end, but to do code coverage, we must directly access
// the main file as part of the test.
func TestGot(t *testing.T) {
	// args is a global in the main package just for testing

	args = "-t got -i -o github.com/goradd/got/test/template -I github.com/goradd/got/test/src/inc2:github.com/goradd/got/test/src/inc -d github.com/goradd/got/test/src"

	main()

	testPath := filepath.Join(`github.com/goradd/got/test`)
	runnerPath := filepath.Join(testPath, "runner", "runner.go")
	comparePath := filepath.Join(testPath, "compare")
	expectedPath := filepath.Join(testPath, "expected")
	sys.ExecuteShellCommand("go run " + runnerPath + " " + comparePath)

	// compare the outputs and report differences

	files,_ := filepath.Glob(expectedPath + string(os.PathSeparator) + "*.out")

	for _,file := range files {
		compare,_ := ioutil.ReadFile(file)
		if expected,err := ioutil.ReadFile(filepath.Join(comparePath, filepath.Base(file))); err != nil {
			t.Error(err)
		} else if bytes.Compare(expected, compare) != 0 {
			t.Errorf("File %s is not a match.", filepath.Base(file))
		}
	}
}

