package got

// Sets up the token map, mapping tokens to items. Allows us to use a variety of different tokens for the
// same item, including translations

//go:generate stringer -type=itemType
type itemType int

type item struct {
	typ        itemType
	escaped    bool
	withError  bool
	translate  bool
	htmlBreaks bool   // adds html break tags in exchange for newlines
	val        string // filled in by lexer after initialization
	newline	   bool   // a run of text should start on a new line
}

const (
	tokEndBlock = "{{end}}"
	tokEnd      = "}}" // must check for a white space before it
)

const (
	itemIgnore itemType = iota
	itemError
	itemStrictBlock
	itemNamedBlock
	itemEndBlock   // ends blocks
	itemSubstitute // substitutes a named block
	itemInclude    // immediately includes another file during lexing

	itemEnd
	itemConvert
	itemGo
	itemText

	itemRun // The run of text that belongs to the previous tag

	itemString
	itemBool
	itemInt
	itemUInt
	itemFloat
	itemInterface
	itemBytes

	itemComment
	itemEOF
	itemBackup

	itemIf
	itemElse
	itemFor
)

var tokens map[string]item

func init() {
	tokens = make(map[string]item)

	tokens["{{!s"] = item{typ: itemString, escaped: true, withError: false}
	tokens["{{!="] = item{typ: itemString, escaped: true, withError: false}
	tokens["{{!h"] = item{typ: itemString, escaped: true, withError: false, htmlBreaks: true}
	tokens["{{!string"] = item{typ: itemString, escaped: true, withError: false}
	tokens["{{="] = item{typ: itemString, escaped: false, withError: false}
	tokens["{{s"] = item{typ: itemString, escaped: false, withError: false}
	tokens["{{string"] = item{typ: itemString, escaped: false, withError: false}
	tokens["{{!se"] = item{typ: itemString, escaped: true, withError: true}
	tokens["{{!=e"] = item{typ: itemString, escaped: true, withError: true}
	tokens["{{!string,err"] = item{typ: itemString, escaped: true, withError: true}
	tokens["{{=e"] = item{typ: itemString, escaped: false, withError: true}
	tokens["{{se"] = item{typ: itemString, escaped: false, withError: true}
	tokens["{{string,err"] = item{typ: itemString, escaped: false, withError: true}

	// It doesn't make sense to html escape booleans, integers, etc
	tokens["{{b"] = item{typ: itemBool, escaped: false, withError: false}
	tokens["{{bool"] = item{typ: itemBool, escaped: false, withError: false}
	tokens["{{be"] = item{typ: itemBool, escaped: false, withError: true}
	tokens["{{bool,err"] = item{typ: itemBool, escaped: false, withError: true}

	tokens["{{i"] = item{typ: itemInt, escaped: false, withError: false}
	tokens["{{int"] = item{typ: itemInt, escaped: false, withError: false}
	tokens["{{ie"] = item{typ: itemInt, escaped: false, withError: true}
	tokens["{{int,err"] = item{typ: itemInt, escaped: false, withError: true}

	tokens["{{u"] = item{typ: itemUInt, escaped: false, withError: false}
	tokens["{{uint"] = item{typ: itemUInt, escaped: false, withError: false}
	tokens["{{ue"] = item{typ: itemUInt, escaped: false, withError: true}
	tokens["{{uint,err"] = item{typ: itemUInt, escaped: false, withError: true}

	tokens["{{f"] = item{typ: itemFloat, escaped: false, withError: false}
	tokens["{{float"] = item{typ: itemFloat, escaped: false, withError: false}
	tokens["{{fe"] = item{typ: itemFloat, escaped: false, withError: true}
	tokens["{{float,err"] = item{typ: itemFloat, escaped: false, withError: true}

	tokens["{{!w"] = item{typ: itemBytes, escaped: true, withError: false}
	tokens["{{!bytes"] = item{typ: itemBytes, escaped: true, withError: false}
	tokens["{{w"] = item{typ: itemBytes, escaped: false, withError: false}
	tokens["{{bytes"] = item{typ: itemBytes, escaped: false, withError: false}
	tokens["{{!we"] = item{typ: itemBytes, escaped: true, withError: true}
	tokens["{{!bytes,err"] = item{typ: itemBytes, escaped: true, withError: true}
	tokens["{{we"] = item{typ: itemBytes, escaped: false, withError: true}
	tokens["{{bytes,err"] = item{typ: itemBytes, escaped: false, withError: true}

	tokens["{{!v"] = item{typ: itemInterface, escaped: true, withError: false}
	tokens["{{!stringer"] = item{typ: itemInterface, escaped: true, withError: false}
	tokens["{{v"] = item{typ: itemInterface, escaped: false, withError: false}
	tokens["{{interface"] = item{typ: itemInterface, escaped: false, withError: false}
	tokens["{{!ve"] = item{typ: itemInterface, escaped: true, withError: true}
	tokens["{{!stringer,err"] = item{typ: itemInterface, escaped: true, withError: true}
	tokens["{{ve"] = item{typ: itemInterface, escaped: false, withError: true}
	tokens["{{stringer,err"] = item{typ: itemInterface, escaped: false, withError: true}

	tokens["{{#"] = item{typ: itemComment}
	tokens["{{//"] = item{typ: itemComment}

	tokens["{{"] = item{typ: itemText, escaped: false, withError: false}
	tokens["{{!"] = item{typ: itemText, escaped: true, withError: false}
	tokens["{{esc"] = item{typ: itemText, escaped: true, withError: false}

	tokens["{{h"] = item{typ: itemConvert}
	tokens["{{html"] = item{typ: itemConvert}

	// go code straight pass through
	tokens["{{g"] = item{typ: itemGo}
	tokens["{{go"] = item{typ: itemGo}
	tokens["{{e"] = item{typ: itemGo, withError: true}
	tokens["{{err"] = item{typ: itemGo, withError: true}

	tokens["{{begin"] = item{typ: itemStrictBlock}
	tokens["{{end}}"] = item{typ: itemEndBlock}
	tokens["{{define"] = item{typ: itemNamedBlock} // must follow with a name and a close tag
	tokens["{{<"] = item{typ: itemNamedBlock}      // must follow with a name and a close tag

	tokens["{{>"] = item{typ: itemSubstitute}   // must follow with a name and a close tag
	tokens["{{put"] = item{typ: itemSubstitute} // must follow with a name and a close tag

	tokens["{{!t"] = item{typ: itemText, escaped: true, translate: true}
	tokens["{{!translate"] = item{typ: itemText, escaped: true, translate: true}
	tokens["{{t"] = item{typ: itemText, escaped: false, translate: true}
	tokens["{{translate"] = item{typ: itemText, escaped: false, translate: true}

	tokens["{{:"] = item{typ: itemInclude}       // must follow with a file name
	tokens["{{include"] = item{typ: itemInclude} // must follow with a file name
	tokens["{{:h"] = item{typ: itemInclude, escaped:true, withError: false, htmlBreaks:true}       // must follow with a file name
	tokens["{{includeAsHtml"] = item{typ: itemInclude, escaped:true, withError: false, htmlBreaks:true} // must follow with a file name
	tokens["{{:!"] = item{typ: itemInclude, escaped:true, withError: false, htmlBreaks:false}       // must follow with a file name
	tokens["{{includeEscaped"] = item{typ: itemInclude, escaped:true, withError: false, htmlBreaks:false} // must follow with a file name


	tokens["{{-"] = item{typ: itemBackup}      // Can be followed by a number to indicate how many chars to backup
	tokens["{{backup"] = item{typ: itemBackup} // Can be followed by a number to indicate how many chars to backup

	tokens["{{if"] = item{typ: itemIf} // Converted to a go statement
	tokens["{{else"] = item{typ: itemElse}

	tokens["{{for"] = item{typ: itemFor} // Converted to a go statement

	tokens["}}"] = item{typ: itemEnd} // need to check this for white space BEFORE instead of after.
}
