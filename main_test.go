package main

import (
	"bytes"
	"fmt"
	"github.com/goradd/gofile/pkg/sys"
	"github.com/goradd/got/internal/got"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// This file runs the tests found in the test directory. It is set up so that code coverage can be checked as well.
// Doing this is a little tricky, since got generates go code that then gets compiled and run again. Each part of the
// process may generate errors. We test the process from end to end, but to do code coverage, we must directly access
// the main file as part of the test.
func TestGot(t *testing.T) {
	// args is a global in the main package just for testing

	resetTemplates()

	args = "-t got -i -o github.com/goradd/got/test/template -I github.com/goradd/got/test/src/inc2:github.com/goradd/got/test/src/inc:github.com/goradd/got/test/src/inc/testInclude4.inc -d github.com/goradd/got/test/src"

	main()

	testPath := filepath.Join(`./test`)
	runnerPath := filepath.Join(testPath, "runner", "runner.go")
	comparePath := filepath.Join(testPath, "compare")
	expectedPath := filepath.Join(testPath, "expected")
	cmd := "go run " + runnerPath + " " + comparePath
	if _, err := sys.ExecuteShellCommand(cmd); err != nil {
		if e, ok := err.(*exec.Error); ok {
			_, _ = fmt.Fprintln(os.Stderr, "error testing runner.go :"+e.Error()) // perhaps goimports is not installed?
			os.Exit(1)
		} else if err2, ok2 := err.(*exec.ExitError); ok2 {
			// Likely a syntax error in the resulting file
			_, _ = fmt.Fprintln(os.Stderr, string(err2.Stderr))
			os.Exit(1)
		}
	}

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

func Test_badIncludeFail(t *testing.T) {
	resetTemplates()

	ret := got.Run("./test/template", "", false, "", "", []string{"./test/src/failureTests/badInclude.got"})
	assert.Equal(t, 1, ret)
}

func Test_badInclude2Fail(t *testing.T) {
	resetTemplates()

	ret := got.Run("./test/template", "", true, "", "", []string{"./test/src/failureTests/badInclude2.got"})
	assert.Equal(t, 1, ret)
}



func Test_tooManyParams(t *testing.T) {
	resetTemplates()

	ret := got.Run("./test/template", "", false, "", "", []string{"./test/src/failureTests/tooManyParams.got"})
	assert.Equal(t, 1, ret)
}

func Test_badGo2(t *testing.T) {
	resetTemplates()

	ret := got.Run("./test/template", "", true, "", "", []string{"./test/src/failureTests/badGo.got"})
	assert.Equal(t, 1, ret)
}

func TestInfo(t *testing.T) {
	resetTemplates()

	// args is a global in the main package just for testing

	args = "testEmpty"

	main()

}

func resetTemplates() {
	files,_ := filepath.Glob("./test/template/*.go")
	for _,f := range files {
		_ = os.Remove(f)
	}
}
