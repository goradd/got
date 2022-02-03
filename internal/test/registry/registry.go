package registry

import (
	"io"
)

type TestF func(io.Writer) error

type Test struct {
	F    TestF
	Name string
}

var Tests []Test


func RegisterTest (f TestF, name string) {
	Tests = append(Tests, Test{f,name})
}

