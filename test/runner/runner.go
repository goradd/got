package main

import (
	"bytes"
	"github.com/goradd/got/test/template"
	"io/ioutil"
	"os"
	"path/filepath"
)

func main() {
	err := RunTemplates(os.Args[1])
	if err != nil {
		os.Stdout.WriteString(err.Error())
	}
}


func RunTemplates(outDir string) error {
	buf := bytes.Buffer{}
	var err error

	for _,test := range template.Tests {
		err = test.F(&buf)
		if err != nil {
			return err
		}
		writeFile(buf.Bytes(), test.Name + ".out", outDir)
		buf.Reset()
	}
	return nil
}

func writeFile(b []byte, file string, outDir string) {

	dir := outDir
	dir, _ = filepath.Abs(dir)
	file = filepath.Join(outDir, file)

	ioutil.WriteFile(file, b, os.ModePerm)
}
