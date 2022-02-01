package {{package}}

import (
	"io"
	"fmt"
)


func {{name}}(_w io.Writer) (err error) {

    {{body}}
    return
}

func init() {
    registerTest({{name}}, "{{name}}")
}