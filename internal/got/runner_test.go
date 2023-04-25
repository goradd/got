package got

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_getRecursiveDirectories(t *testing.T) {
	dirs, _ := getRecursiveDirectories("../testdata")
	assert.Len(t, dirs, 13)
}

func Test_fileIsNewer(t *testing.T) {
	// test 2nd file missing
	r := fileIsNewer("../testdata/template/stub.go", "../testdata/template/stub2.go")
	assert.True(t, r)

	// test 1st file missing
	r = fileIsNewer("../testdata/template/stub2.go", "../testdata/template/stub.go")
	assert.False(t, r)

	// test same file is false
	r = fileIsNewer("../testdata/template/stub.go", "../testdata/template/stub.go")
	assert.False(t, r)

}
