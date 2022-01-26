package got

import (
	"fmt"
	"github.com/goradd/got/got"
	"io"
	"os"
)

type  astType struct {

}


func buildAst(inFile string) (*astType, error) {
	l := lexFile(inFile)

		s := got.Parse(l)

		return s
	}

	return nil
}

func runAsts(outPath string, asts ...*astType ) {

}