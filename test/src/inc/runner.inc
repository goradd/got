package {{package}}

import (
	"bytes"
	"fmt"
)


func {{name}}(buf *bytes.Buffer) (err error) {

    {{body}}
    return
}

func init() {
    registerTest({{name}}, "{{name}}")
}