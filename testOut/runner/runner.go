package main

import (
	".."
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	buf := bytes.Buffer{}

	testOut.TestInclude(&buf)
	writeFile(buf.Bytes(), "testInclude", "..")

	testOut.TestStatic(&buf)
	writeFile(buf.Bytes(), "testStatic", "..")

	testOut.TestSub(&buf)
	writeFile(buf.Bytes(), "testSub", "..")

	testOut.TestVars(&buf)
	writeFile(buf.Bytes(), "testVars", "..")

}

func writeFile(b []byte, file string, outDir string) {

	i := strings.LastIndex(file, ".")

	dir := filepath.Dir(file)
	dir, _ = filepath.Abs(dir)
	file = filepath.Base(file)

	if i < 0 {
		file = file + ".out"
	} else {
		file = file[:i] + ".out"
	}

	if outDir != "" {
		dir = outDir
	}

	if dir != "/" {
		dir = dir + "/"
	}
	file = dir + file

	ioutil.WriteFile(file, b, os.ModePerm)

}
