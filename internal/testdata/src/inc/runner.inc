package {{package}}

import (
	"io"
	"fmt"
	"github.com/goradd/got/internal/testdata/registry"
	{{>? imports}}
)


func {{name}}(_w io.Writer) (err error) {

    {{body}}
    return
}

func init() {
    registry.RegisterTest({{name}}, "{{name}}")
}