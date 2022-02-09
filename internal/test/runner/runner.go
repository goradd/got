package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/goradd/got/internal/test/registry"
	_ "github.com/goradd/got/internal/test/template"
)

func main() {
	err := runTemplates(os.Args[1])
	if err != nil {
		os.Stdout.WriteString(err.Error())
	}
}

func runTemplates(outDir string) error {
	buf := bytes.Buffer{}
	var err error

	for _, test := range registry.Tests {
		err = test.F(&buf)
		if err != nil {
			return err
		}
		writeFile(buf.Bytes(), test.Name+".out", outDir)
		buf.Reset()
	}
	return nil
}

func writeFile(b []byte, file string, outDir string) {

	dir := outDir
	dir, _ = filepath.Abs(dir)
	file = filepath.Join(dir, file)

	_ = ioutil.WriteFile(file, b, os.ModePerm)
}
