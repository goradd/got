package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goradd/gofile/pkg/sys"
	"github.com/goradd/got/internal/got"
	"github.com/stretchr/testify/assert"
)

// This file runs the tests found in the test directory. It is set up so that code coverage can be checked as well.
// Doing this is a little tricky, since got generates go code that then gets compiled and run again. Each part of the
// process may generate errors. We test the process from end to end, but to do code coverage, we must directly access
// the main file as part of the test.
func TestGot(t *testing.T) {
	// args is a global in the main package just for testing
	testPath := filepath.Join(`./internal`, `testdata`)
	runnerPath := filepath.Join(testPath, "runner", "runner.go")
	comparePath := filepath.Join(testPath, "compare")
	expectedPath := filepath.Join(testPath, "expected")
	cmd := "go run " + runnerPath + " " + comparePath
	curDir, _ := os.Getwd()

	resetTemplates()

	args = "-t got -i -o github.com/goradd/got/internal/testdata/template -I github.com/goradd/got/internal/testdata/src/inc2:github.com/goradd/got/internal/testdata/src/inc:github.com/goradd/got/internal/testdata/src/inc/testInclude4.inc -d github.com/goradd/got/internal/testdata/src"

	main()

	// main seems to be changing working dir
	_ = os.Chdir(curDir)

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

	files, _ := filepath.Glob(expectedPath + string(os.PathSeparator) + "*.out")

	for _, file := range files {
		compare, _ := os.ReadFile(file)
		if expected, err := os.ReadFile(filepath.Join(comparePath, filepath.Base(file))); err != nil {
			t.Error(err)
		} else {
			expected = bytes.Replace(expected, []byte("\r\n"), []byte("\n"), -1)
			compare = bytes.Replace(compare, []byte("\r\n"), []byte("\n"), -1)
			if bytes.Compare(expected, compare) != 0 {
				t.Errorf("File %s is not a match.", filepath.Base(file))
			}
		}
	}
}

// TestRecursiveGot is an alternate test for testing some of the command line options.
func TestRecursiveGot(t *testing.T) {
	// args is a global in the main package just for testing
	outPath1 := filepath.Join(`./internal`, `testdata`, `src`, `recurse`)
	outPath2 := filepath.Join(outPath1, `rdir`)
	curDir, _ := os.Getwd()

	resetTemplates()

	var b bytes.Buffer
	got.OutWriter = &b

	args = "-t got -r -v -d github.com/goradd/got/internal/testdata/src/recurse"

	main()

	// main seems to be changing working dir
	_ = os.Chdir(curDir)

	// verify outputs were created

	files1, _ := filepath.Glob(outPath1 + string(os.PathSeparator) + "*.go")
	files2, _ := filepath.Glob(outPath2 + string(os.PathSeparator) + "*.go")

	assert.Len(t, files1, 1)
	assert.Equal(t, "/r1.tpl.go", strings.TrimPrefix(files1[0], outPath1))
	assert.Len(t, files2, 1)
	assert.Equal(t, "/r2.tpl.go", strings.TrimPrefix(files2[0], outPath2))
	assert.True(t, strings.HasPrefix(b.String(), "Processing"))

	// Running this again shows that files were not processed
	b.Reset()
	err := got.Run("",
		"got",
		false,
		"",
		"github.com/goradd/got/internal/testdata/src/recurse",
		nil,
		true,
		true,
		false)
	assert.NoError(t, err)
	assert.False(t, strings.HasPrefix(b.String(), "Processing"))

	// Running it again with force on shows that files were processed
	b.Reset()
	err = got.Run("",
		"got",
		false,
		"",
		"github.com/goradd/got/internal/testdata/src/recurse",
		nil,
		true,
		true,
		true)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(b.String(), "Processing"))

	resetTemplates()
}

func Test_badFlags1(t *testing.T) {
	resetTemplates()

	err := got.Run("./internal/testdata/template",
		"",
		false,
		"",
		"",
		nil,
		false,
		true,
		true)
	assert.Error(t, err)
}

func Test_badIncludeFail(t *testing.T) {
	resetTemplates()

	err := got.Run("./internal/testdata/template",
		"",
		false,
		"",
		"",
		[]string{"./internal/testdata/src/failureTests/badInclude.tpl.got"},
		false,
		false,
		true)
	assert.Error(t, err)
}

func Test_badInclude2Fail(t *testing.T) {
	resetTemplates()

	err := got.Run("./internal/testdata/template",
		"",
		true,
		"",
		"",
		[]string{"./internal/testdata/src/failureTests/badInclude2.tpl.got"},
		false,
		false,
		true)

	assert.Error(t, err)
}

func Test_tooManyParams(t *testing.T) {
	resetTemplates()

	err := got.Run("./internal/testdata/template",
		"",
		false,
		"",
		"",
		[]string{"./internal/testdata/src/failureTests/tooManyParams.tpl.got"},
		false,
		false,
		true)

	assert.Error(t, err)
}

func Test_badGo2(t *testing.T) {
	resetTemplates()

	err := got.Run("./internal/testdata/template",
		"",
		true,
		"",
		"",
		[]string{"./internal/testdata/src/failureTests/badGo.tpl.got"},
		false,
		false,
		true)

	assert.Error(t, err)
}

func Test_badBlock(t *testing.T) {
	resetTemplates()

	err := got.Run("./internal/testdata/template",
		"",
		true,
		"",
		"",
		[]string{"./internal/testdata/src/failureTests/badBlock.tpl.got"},
		false,
		false,
		true)

	assert.Error(t, err)
}

func Test_tooManyEnds(t *testing.T) {
	resetTemplates()

	err := got.Run("./internal/testdata/template",
		"",
		true,
		"",
		"",
		[]string{"./internal/testdata/src/failureTests/tooManyEnds.tpl.got"},
		false,
		false,
		true)

	assert.Error(t, err)
}

func TestInfo(t *testing.T) {
	resetTemplates()

	// args is a global in the main package just for testing

	args = "testEmpty"

	main()
}

func resetTemplates() {
	files, _ := filepath.Glob("./internal/testdata/template/*.tpl.go")
	for _, f := range files {
		_ = os.Remove(f)
	}

	files, _ = filepath.Glob("./internal/testdata/src/recurse/*.tpl.go")
	for _, f := range files {
		_ = os.Remove(f)
	}

	files, _ = filepath.Glob("./internal/testdata/src/recurse/rdir/*.tpl.go")
	for _, f := range files {
		_ = os.Remove(f)
	}

}
