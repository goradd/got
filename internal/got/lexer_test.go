package got

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSplitParams(t *testing.T) {
	var p []string
	var err error

	p, _ = splitParams("test")
	assert.Equal(t, []string{"test"}, p)

	p, _ = splitParams("test, test2")
	assert.Equal(t, []string{"test", "test2"}, p)

	p, _ = splitParams("test,test2")
	assert.Equal(t, []string{"test", "test2"}, p)
	p, _ = splitParams("\"test me\",test2")
	assert.Equal(t, []string{"test me", "test2"}, p)

	p, _ = splitParams("\"test, me\",test2")
	assert.Equal(t, []string{"test, me", "test2"}, p)

	p, _ = splitParams(`"test\", me",test2`)
	assert.Equal(t, []string{`test", me`, "test2"}, p)

	p, _ = splitParams(`"test\", me","test2"`)
	assert.Equal(t, []string{`test", me`, "test2"}, p)

	p, _ = splitParams(`"test\", me"," test2 "`)
	assert.Equal(t, []string{`test", me`, " test2 "}, p)

	p, _ = splitParams(`"test\", me\","," test2 "`)
	assert.Equal(t, []string{`test", me",`, " test2 "}, p)

	p, err = splitParams(`"test\", `)
	assert.Error(t, err)

	p, err = splitParams(`test", `)
	assert.Error(t, err)

}
/*
func TestProcessParams(t *testing.T) {
	var out string
	//var err error

	out, _ = processParams("a $1", "b")
	assert.Equal(t, "a b", out)

	out, _ = processParams("a $1", "b,c")
	assert.Equal(t, "a b", out)

	out, _ = processParams(`a $1 "$2"`, "b,c")
	assert.Equal(t, `a b "c"`, out)

	// Test that parameters not included default to empty string
	out, _ = processParams(`a $1 "$2"`, "b")
	assert.Equal(t, `a b ""`, out)

}
*/
