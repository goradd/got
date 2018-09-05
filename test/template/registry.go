package template

import "bytes"

type TestF func(buf *bytes.Buffer) error

type Test struct {
	F    TestF
	Name string
}

var Tests []Test


func registerTest (f TestF, name string) {
	Tests = append(Tests, Test{f,name})
}

