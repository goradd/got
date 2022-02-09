package registry

import (
	"io"
)

// TestF is a function used by the test harness
type TestF func(io.Writer) error

// Test represents a GoT test
type Test struct {
	F    TestF
	Name string
}

// Tests is the collection of registered tests
var Tests []Test

// RegisterTest is used by the tests to register themselves with the test harness
func RegisterTest(f TestF, name string) {
	Tests = append(Tests, Test{f, name})
}
