package got

import "fmt"

// Sets up the token map, mapping tokens to items. Allows us to use a variety of different tokens for the
// same tokenItem, including translations

type tokenType int

type locationRef struct {
	fileName  string
	blockName string
	lineNum   int
	offset    int
}

type tokenItem struct {
	typ        tokenType
	escaped    bool
	optional   bool
	withError  bool
	translate  bool
	htmlBreaks bool   // adds html break tags in exchange for newlines
	val        string // filled in by lexer after initialization
	callStack  []locationRef
	childItems []tokenItem          // Filled in during the parsing step to build the ast
	params     map[string]tokenItem // Filled in during the parsing step to build the ast
}

const (
	tokEnd   = "}}"
	tokBegin = "{{"
)

const (
	itemEOF tokenType = iota // 0 so that its the default after the channel is closed
	itemError
	itemStrictBlock
	itemNamedBlock
	itemEndBlock   // ends blocks
	itemSubstitute // substitutes a named block
	itemInclude    // immediately includes another file during lexing

	itemEnd
	itemGo
	itemGoErr
	itemText

	itemRun // A run of text

	itemString
	itemBool
	itemInt
	itemUInt
	itemFloat
	itemInterface
	itemBytes
	itemGoLiteral
	itemGoType
	itemGoTypeWithPackage

	itemComment

	itemIf
	itemElse   // only used by parser
	itemElseIf // only used by parser
	itemFor

	itemJoin
	itemParam
)

var tokens map[string]tokenItem

func init() {
	tokens = make(map[string]tokenItem)

	tokens["{{!s"] = tokenItem{typ: itemString, escaped: true, withError: false}
	tokens["{{!="] = tokenItem{typ: itemString, escaped: true, withError: false}
	tokens["{{!h"] = tokenItem{typ: itemString, escaped: true, withError: false, htmlBreaks: true}
	tokens["{{!string"] = tokenItem{typ: itemString, escaped: true, withError: false}
	tokens["{{="] = tokenItem{typ: itemString, escaped: false, withError: false}
	tokens["{{s"] = tokenItem{typ: itemString, escaped: false, withError: false}
	tokens["{{string"] = tokenItem{typ: itemString, escaped: false, withError: false}
	tokens["{{!se"] = tokenItem{typ: itemString, escaped: true, withError: true}
	tokens["{{!=e"] = tokenItem{typ: itemString, escaped: true, withError: true}
	tokens["{{!string,err"] = tokenItem{typ: itemString, escaped: true, withError: true}
	tokens["{{=e"] = tokenItem{typ: itemString, escaped: false, withError: true}
	tokens["{{se"] = tokenItem{typ: itemString, escaped: false, withError: true}
	tokens["{{string,err"] = tokenItem{typ: itemString, escaped: false, withError: true}

	// It doesn't make sense to html escape booleans, integers, etc
	tokens["{{b"] = tokenItem{typ: itemBool, escaped: false, withError: false}
	tokens["{{bool"] = tokenItem{typ: itemBool, escaped: false, withError: false}
	tokens["{{be"] = tokenItem{typ: itemBool, escaped: false, withError: true}
	tokens["{{bool,err"] = tokenItem{typ: itemBool, escaped: false, withError: true}

	tokens["{{i"] = tokenItem{typ: itemInt, escaped: false, withError: false}
	tokens["{{int"] = tokenItem{typ: itemInt, escaped: false, withError: false}
	tokens["{{ie"] = tokenItem{typ: itemInt, escaped: false, withError: true}
	tokens["{{int,err"] = tokenItem{typ: itemInt, escaped: false, withError: true}

	tokens["{{u"] = tokenItem{typ: itemUInt, escaped: false, withError: false}
	tokens["{{uint"] = tokenItem{typ: itemUInt, escaped: false, withError: false}
	tokens["{{ue"] = tokenItem{typ: itemUInt, escaped: false, withError: true}
	tokens["{{uint,err"] = tokenItem{typ: itemUInt, escaped: false, withError: true}

	tokens["{{f"] = tokenItem{typ: itemFloat, escaped: false, withError: false}
	tokens["{{float"] = tokenItem{typ: itemFloat, escaped: false, withError: false}
	tokens["{{fe"] = tokenItem{typ: itemFloat, escaped: false, withError: true}
	tokens["{{float,err"] = tokenItem{typ: itemFloat, escaped: false, withError: true}

	tokens["{{!w"] = tokenItem{typ: itemBytes, escaped: true, withError: false}
	tokens["{{!bytes"] = tokenItem{typ: itemBytes, escaped: true, withError: false}
	tokens["{{w"] = tokenItem{typ: itemBytes, escaped: false, withError: false}
	tokens["{{bytes"] = tokenItem{typ: itemBytes, escaped: false, withError: false}
	tokens["{{!we"] = tokenItem{typ: itemBytes, escaped: true, withError: true}
	tokens["{{!bytes,err"] = tokenItem{typ: itemBytes, escaped: true, withError: true}
	tokens["{{we"] = tokenItem{typ: itemBytes, escaped: false, withError: true}
	tokens["{{bytes,err"] = tokenItem{typ: itemBytes, escaped: false, withError: true}

	tokens["{{!v"] = tokenItem{typ: itemInterface, escaped: true, withError: false}
	tokens["{{!stringer"] = tokenItem{typ: itemInterface, escaped: true, withError: false}
	tokens["{{v"] = tokenItem{typ: itemInterface, escaped: false, withError: false}
	tokens["{{interface"] = tokenItem{typ: itemInterface, escaped: false, withError: false}
	tokens["{{!ve"] = tokenItem{typ: itemInterface, escaped: true, withError: true}
	tokens["{{!stringer,err"] = tokenItem{typ: itemInterface, escaped: true, withError: true}
	tokens["{{ve"] = tokenItem{typ: itemInterface, escaped: false, withError: true}
	tokens["{{stringer,err"] = tokenItem{typ: itemInterface, escaped: false, withError: true}
	tokens["{{L"] = tokenItem{typ: itemGoLiteral, escaped: false, withError: false}
	tokens["{{T"] = tokenItem{typ: itemGoType, escaped: false, withError: false}
	tokens["{{PT"] = tokenItem{typ: itemGoTypeWithPackage, escaped: false, withError: false}

	tokens["{{#"] = tokenItem{typ: itemComment}
	tokens["{{//"] = tokenItem{typ: itemComment}

	tokens["{{"] = tokenItem{typ: itemText, escaped: false, withError: false}
	tokens["{{!"] = tokenItem{typ: itemText, escaped: true, withError: false}
	tokens["{{esc"] = tokenItem{typ: itemText, escaped: true, withError: false}
	tokens["{{h"] = tokenItem{typ: itemText, escaped: true, htmlBreaks: true}
	tokens["{{html"] = tokenItem{typ: itemText, escaped: true, htmlBreaks: true}

	// go code straight pass through
	tokens["{{g"] = tokenItem{typ: itemGo}
	tokens["{{go"] = tokenItem{typ: itemGo}
	tokens["{{e"] = tokenItem{typ: itemGoErr, withError: true}
	tokens["{{err"] = tokenItem{typ: itemGoErr, withError: true}

	tokens["{{begin"] = tokenItem{typ: itemStrictBlock}
	tokens["{{define"] = tokenItem{typ: itemNamedBlock} // must follow with a name and a close tag
	tokens["{{<"] = tokenItem{typ: itemNamedBlock}      // must follow with a name and a close tag

	tokens["{{>"] = tokenItem{typ: itemSubstitute}                    // must follow with a name and a close tag
	tokens["{{put"] = tokenItem{typ: itemSubstitute}                  // must follow with a name and a close tag
	tokens["{{>?"] = tokenItem{typ: itemSubstitute, optional: true}   // must follow with a name and a close tag
	tokens["{{put?"] = tokenItem{typ: itemSubstitute, optional: true} // must follow with a name and a close tag

	tokens["{{!t"] = tokenItem{typ: itemText, escaped: true, translate: true}
	tokens["{{!translate"] = tokenItem{typ: itemText, escaped: true, translate: true}
	tokens["{{t"] = tokenItem{typ: itemText, escaped: false, translate: true}
	tokens["{{translate"] = tokenItem{typ: itemText, escaped: false, translate: true}

	tokens["{{:"] = tokenItem{typ: itemInclude}                                                                  // must follow with a file name
	tokens["{{include"] = tokenItem{typ: itemInclude}                                                            // must follow with a file name
	tokens["{{:h"] = tokenItem{typ: itemInclude, escaped: true, withError: false, htmlBreaks: true}              // must follow with a file name
	tokens["{{includeAsHtml"] = tokenItem{typ: itemInclude, escaped: true, withError: false, htmlBreaks: true}   // must follow with a file name
	tokens["{{:!"] = tokenItem{typ: itemInclude, escaped: true, withError: false, htmlBreaks: false}             // must follow with a file name
	tokens["{{includeEscaped"] = tokenItem{typ: itemInclude, escaped: true, withError: false, htmlBreaks: false} // must follow with a file name

	tokens["{{if"] = tokenItem{typ: itemIf} // Outputs a go "if" statement
	tokens["{{if}}"] = tokenItem{typ: itemEndBlock, val: "if"}
	tokens["{{elseif"] = tokenItem{typ: itemEndBlock, val: "elseif"}
	tokens["{{else}}"] = tokenItem{typ: itemEndBlock, val: "else"}

	tokens["{{for"] = tokenItem{typ: itemFor} // Outputs a go "for" statement
	tokens["{{for}}"] = tokenItem{typ: itemEndBlock, val: "for"}

	tokens["{{join"] = tokenItem{typ: itemJoin} // Like a string.Join statement
	tokens["{{join}}"] = tokenItem{typ: itemEndBlock, val: "join"}

	tokens["}}"] = tokenItem{typ: itemEnd}
}

func (t tokenItem) formatError() (s string) {
	if t.typ == itemError {
		s = "*** Error: " + t.val + "\n"
		for _, c := range t.callStack {
			s += "    " + c.formatErrorLine() + "\n"
		}
	}
	return
}

func (r locationRef) formatErrorLine() (s string) {
	if r.blockName != "" {
		s += fmt.Sprintf("Block %s:%d:%d", r.blockName, r.lineNum+1, r.offset)
	} else {
		s += fmt.Sprintf("%s:%d:%d", r.fileName, r.lineNum+1, r.offset)
	}
	return
}
