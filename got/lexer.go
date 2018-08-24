package got

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	"text/scanner"
)

const eof = -1

type Pos int

var IncludePaths []string
var IncludeFiles []string

var namedBlocks map[string]string

func init() {
	namedBlocks = make(map[string]string)
}

func (i item) String() string {
	switch i.typ {
	case itemError:
		return i.val

	case itemEOF:
		return "EOF"

	}

	/*
		if len(i.val) > 10 {
			return fmt.Sprintf("%.10q...", i.val)
		}
	*/

	return fmt.Sprintf("%v: %q\n", i.typ, i.val)
}

type lexer struct {
	fileName  string    // file name being scanned
	blockName string    // named block being scanned
	input     string    // string being scanned
	start     int       // start position of item
	pos       int       // current position
	lastPos   int       // last position of item read
	width     int       // width of last rune
	items     chan item // channel of scanned items
	hasError  bool
	openCount int // Make sure open and close tags are matched
}

type stateFn func(*lexer) stateFn

func (l *lexer) run() {
	for state := lexStart; state != nil; {
		state = state(l)
	}
	l.emitType(itemEOF)
	close(l.items)
}

// nextItem returns the next item from the input.
// Called by the parser, not in the lexing goroutine.
func (l *lexer) nextItem() item {

	select {
	case i := <-l.items:
		l.lastPos = l.pos
		return i
	case <-time.After(10 * time.Second): // Internal error? We are supposed to detect EOF situations before we get here
		//close(l.items)
		return item{typ: itemError, val: "*** Internal error at line " + strconv.Itoa(l.getLine()) + " read past end of file " + l.fileName + ". Are you missing an end tag?"}
	}
}

func Lex(input string, fileName string) *lexer {
	l := &lexer{
		input:    input,
		fileName: fileName,
		items:    make(chan item),
	}
	go l.run()
	return l
}

func (l *lexer) emitType(t itemType) {
	item := item{typ: t, val: l.input[l.start:l.pos]}
	l.items <- item
	l.start = l.pos
	//fmt.Printf("%v", item)

}

func (l *lexer) emit(i item) {
	i.val = l.input[l.start:l.pos]
	l.items <- i
	l.start = l.pos
	//fmt.Printf("%v", item)

}

func (l *lexer) emitRun(prefix string, suffix string) {
	var i = item{typ: itemRun, val: prefix + l.input[l.start:l.pos] + suffix}
	l.items <- i
	l.start = l.pos
	//fmt.Printf("%v", item)

}

// Starting state. We start in GO mode.
func lexStart(l *lexer) stateFn {
	//l.emitType(itemGo)
	return l.lexGo(nil)
}

// We are pointing to the start of an unknown tag
func (l *lexer) lexTag(priorState stateFn) stateFn {
	pos := l.pos
	a := l.acceptTag()

	var i item
	var ok bool

	if i, ok = tokens[a]; !ok {
		// We could not find a tag, so it could be one of two things:
		// - A custom tag we defined, or
		// - A compressed "interface" tag of the form: {{SomeGoCodeWithValue}}, which will be somewhat similar to the built-in go template engine
		if _, ok = namedBlocks[a[2:]]; ok {
			// its a defined block
			l.pos = pos
			l.next()
			l.next()
			l.ignore()
			i = tokens["{{>"]
		} else {
			// we are going to treat it as a go value
			// TODO: Figure out how to discern between a potential go value and an attempt to use an unknown tag
			l.pos = pos
			l.next()
			l.next()
			l.ignore()
			i = tokens["{{v"] // a slow but convenient tag
		}
	}

	switch i.typ {
	case itemStrictBlock:
		l.emit(i)
		l.ignoreOneSpace()
		return l.lexStrictBlock(priorState)

	case itemInclude:
		return l.lexInclude(priorState)

	case itemNamedBlock:
		return l.lexNamedBlock(priorState)

	case itemBackup:
		return l.lexBackup(priorState)

	case itemSubstitute:
		return l.lexSubstitute(priorState)

	case itemComment:
		return l.lexComment(priorState)

	case itemGo:
		l.emit(i)
		l.ignoreWhiteSpace()
		l.openCount++
		return l.lexGo(priorState)

	case itemConvert:
		l.emit(i)
		l.ignoreWhiteSpace()
		return l.lexConvert(priorState)

	case itemText:
		l.emit(i)
		l.ignoreOneSpace()
		return l.lexText(priorState)

	case itemIf:
		return l.lexIf(priorState)

	case itemElse:
		return l.lexElse(priorState)

	case itemFor:
		return l.lexFor(priorState)

	default:
		l.emit(i)
		l.ignoreWhiteSpace()
		return l.lexValue(priorState)

	}
}

func (l *lexer) lexStrictBlock(nextState stateFn) stateFn {
	l.ignore()
	l.acceptRun()
	endToken := l.currentString()
	if !l.isAtCloseTag() {
		return l.errorf("Expected close tag")
	}
	l.ignoreCloseTag()
	endToken = "{{" + endToken + "}}"

	offset := strings.Index(l.input[l.start:], endToken)
	if offset == -1 {
		return l.errorf("No strict end block found")
	}
	l.pos += offset
	l.emitType(itemRun)
	l.start += len(endToken) // skip end token
	l.pos = l.start
	l.width = 0
	l.emitType(itemEnd)
	return nextState
}

func (l *lexer) lexInclude(nextState stateFn) stateFn {
	l.ignore()
	l.acceptRun()
	fileName := l.currentString()
	if !l.isAtCloseTag() {
		return l.errorf("Expected close tag")
	}
	l.ignoreCloseTag()

	fileName = strings.TrimSpace(fileName)
	fileName = strings.Trim(fileName, "\"")

	// Add relative processing from the current path
	dir := filepath.Dir(l.fileName)
	if dir != "." {
		fileName = dir + "/" + fileName
	}

	log.Println("Opening " + fileName)

	// find the file from the include paths
	var buf []byte
	var err error
	if len(IncludePaths) > 0 {
		for _, path := range IncludePaths {
			if buf, err = ioutil.ReadFile(path + "/" + fileName); err == nil {
				break
			}
			if !os.IsNotExist(err) {
				return l.errorf("File read error: %s", err.Error())
			}
		}
	} else {
		buf, err = ioutil.ReadFile(fileName)
	}

	if os.IsNotExist(err) {
		s := "Could not find include file \"" + fileName + "\""
		if len(IncludePaths) > 0 {
			s += " in directories " + strings.Join(IncludePaths, ";")
		}
		return l.errorf(s)
	}
	if err != nil {
		return l.errorf("File read error: %s", err.Error())
	}

	s := string(buf[:])

	l2 := &lexer{
		input:    s,
		fileName: fileName,
		items:    l.items,
	}
	for state := lexStart; state != nil; {
		state = state(l2)
	}

	if l2.hasError {
		return nil
	}
	return nextState
}

// lexValue is going to retrieve go code that returns a value
func (l *lexer) lexValue(nextState stateFn) stateFn {
	l.ignoreWhiteSpace()
	l.acceptRun()

	if l.peek() == eof {
		return l.errorf("Looking for close tag, found end of file.")
	}
	if !l.isAtCloseTag() {
		return l.errorf("Looking for close tag, found %s", l.input[l.pos:l.pos+2])
	}
	l.emitType(itemRun)
	l.ignoreCloseTag()
	l.emitType(itemEnd)

	return nextState
}

func (l *lexer) lexNamedBlock(nextState stateFn) stateFn {
	l.ignoreSpace()
	l.acceptRun()

	if !l.isAtCloseTag() {
		return l.errorf("Looking for close tag, found %s", l.input[l.pos:l.pos+2])
	}
	name := l.currentString()
	l.ignoreCloseTag()

	if strings.ContainsAny(name, " \t\r\n") {
		return l.errorf("Block name cannot contain spaces")
	}

	if _, ok := tokens["{{"+name]; ok {
		return l.errorf("Block name cannot be a tag name. Block name: %s")
	}

	offset := strings.Index(l.input[l.start:], tokEndBlock)
	if offset == -1 {
		return l.errorf("No end block found")
	}

	namedBlocks[name] = l.input[l.start : l.start+offset]
	l.start = l.start + offset + len(tokEndBlock)
	l.pos = l.start

	return nextState
}

func (l *lexer) lexBackup(nextState stateFn) stateFn {
	l.ignoreSpace()
	l.acceptRun()

	if !l.isAtCloseTag() {
		return l.errorf("Looking for close tag, found %s", l.input[l.pos:l.pos+2])
	}

	if l.currentString() != "" {
		if _, err := strconv.ParseUint(l.currentString(), 10, 32); err != nil {
			return l.errorf("Backup tag did not contain numbers only")
		}

	}
	l.emitType(itemBackup)

	l.ignoreCloseTag()
	l.ignoreNewline()

	return nextState
}

func (l *lexer) lexSubstitute(nextState stateFn) stateFn {
	l.ignoreSpace()
	l.acceptTag()
	name := l.currentString()
	l.ignoreSpace()
	l.acceptRun()
	paramString := strings.TrimSpace(l.currentString())

	if !l.isAtCloseTag() {
		return l.errorf("Looking for close tag, found %s", l.input[l.pos:l.pos+2])
	}

	l.ignoreCloseTag()

	var block string
	var ok bool

	if block, ok = namedBlocks[name]; !ok {
		return l.errorf("Named block not found: %s", name)
	}

	// If params are found, do the substitution
	if paramString != "" {
		var err error
		if block, err = processParams(block, paramString); err != nil {
			return l.errorf(err.Error())
		}
	}

	l2 := &lexer{
		input:     block,
		blockName: name,
		items:     l.items,
	}
	for state := nextState; state != nil; {
		state = state(l2)
	}

	if l2.hasError {
		return nil
	}
	return nextState
}

func processParams(in, paramString string) (out string, err error) {
	paramString = strings.TrimSpace(paramString)
	params, err := splitParams(paramString)

	if err != nil {
		return
	}

	var i int
	var s string
	for i, s = range params {
		search := fmt.Sprintf("$%d", i+1)
		in = strings.Replace(in, search, s, -1)
	}

	// Default missing parameters to blanks
	for j := i + 1; j < 9; j++ {
		search := fmt.Sprintf("$%d", j+1)
		if strings.Index(in, search) != -1 {
			in = strings.Replace(in, search, "", -1)
		}
	}
	out = in
	return
}

func splitParams(paramString string) (params []string, err error) {
	var currentItem string

	var s scanner.Scanner
	s.Init(strings.NewReader(paramString))
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		text := s.TokenText()
		if text == "," {
			currentItem = strings.TrimSpace(currentItem)
			if currentItem != "" {
				if currentItem[0] == '"' && currentItem[len(currentItem) - 1] == '"' {
					currentItem = currentItem[1:len(currentItem)-1]
				}
				params = append(params, currentItem)
				currentItem = ""
			}
		} else {
			currentItem += text
		}
	}
	if currentItem != "" {
		if currentItem[0] == '"' && currentItem[len(currentItem)-1] == '"' {
			currentItem = currentItem[1 : len(currentItem)-1]
		}
		params = append(params, currentItem)
	}
	return
/*
	items := strings.Split(paramString, ",")

	// Check to see if we split something surrounded by quotes
	for _, item := range items {
		cleanItem = strings.TrimSpace(item)
		if len(cleanItem) == 0 {
			if currentItem != "" {
				currentItem += "," + item
			} else {
				params = append(params, cleanItem)
			}
		} else if cleanItem[0:1] == "\"" {
			if cleanItem[len(cleanItem)-1:] == "\"" {
				if len(cleanItem) > 1 && item[len(cleanItem)-2:len(cleanItem)-1] != "\\" {
					// an item bounded by quotes, so add it without the quotes
					currentItem = item[1 : len(cleanItem)-1]
					currentItem = cleanEscapedQuotes(currentItem)
					params = append(params, currentItem)
					currentItem = ""
				} else if len(cleanItem) == 1 {
					// a single quote, so either begin or end with a blank item
					if currentItem == "" {
						// start the item
						currentItem = ","
					} else {
						currentItem += ","
						params = append(params, currentItem)
						currentItem = ""
					}
				} else {
					// an item started with a quote, but ended with an escaped quote, so build the string using the original non-cleaned item.
					offset := strings.Index(item, "\"")
					currentItem = cleanEscapedQuotes(item[offset+1:])
				}
			} else {
				// an item started with a quote, but not ended with a quote, so build the item
				offset := strings.Index(item, "\"")
				currentItem = cleanEscapedQuotes(item[offset+1:])
			}
		} else {
			if cleanItem[len(cleanItem)-1:] == "\"" {
				if len(cleanItem) > 1 && cleanItem[len(cleanItem)-2:len(cleanItem)-1] != "\\" {
					// an item ending with a quote, but not started with a quote
					if currentItem != "" {
						lastOffset := strings.LastIndex(item, "\"")
						currentItem += "," + cleanEscapedQuotes(item[:lastOffset])
						params = append(params, currentItem)
						currentItem = ""
					} else {
						err = fmt.Errorf("Defined block parameter ends with a quote but does not start with a quote: %s", item)
						return
					}
				} else {
					// an item ending with an escaped quote, so just include it.
					if currentItem != "" {
						currentItem += "," + cleanEscapedQuotes(item)
					} else {
						params = append(params, cleanItem)
					}
				}
			} else {
				// A normal item
				if currentItem != "" {
					currentItem += "," + cleanEscapedQuotes(item)
				} else {
					params = append(params, cleanItem)
				}
			}
		}
	}

	if currentItem != "" {
		err = fmt.Errorf("Defined block parameter starts with a quote but does not end with a quote: %s", currentItem)
	}
	return
*/
}

func cleanEscapedQuotes(s string) string {
	return strings.Replace(s, "\\\"", "\"", -1)
}

func (l *lexer) lexComment(nextState stateFn) stateFn {
	l.ignoreRun()

	if !l.isAtCloseTag() {
		return l.errorf("Looking for close tag, found %s", l.input[l.pos:l.pos+2])
	}

	l.ignoreCloseTag()

	return nextState
}

func (l *lexer) lexIf(nextState stateFn) stateFn {
	l.emitType(itemGo)
	l.ignoreWhiteSpace()
	l.openCount++
	if l.isAtCloseTag() { // this is a closing tag
		return l.lexGoExtra(nextState, " }\n", "")
	} else {
		return l.lexGoExtra(nextState, " if ", " { ")
	}
}

// lexElse lexes an else tag, which is {{else}}
func (l *lexer) lexElse(nextState stateFn) stateFn {
	l.emitType(itemGo)
	l.ignoreWhiteSpace()
	l.openCount++
	if !l.isAtCloseTag() { // TODO: else if
		return l.errorf("Looking for close tag, found %s", l.input[l.pos:l.pos+2])
	}

	return l.lexGoExtra(nextState, " } else {\n", "")
}

func (l *lexer) lexFor(nextState stateFn) stateFn {
	l.emitType(itemGo)
	l.ignoreWhiteSpace()
	l.openCount++
	if l.isAtCloseTag() { // this is a closing tag
		return l.lexGoExtra(nextState, " }\n", "")
	} else {
		return l.lexGoExtra(nextState, " for ", " { ")
	}
}

func (l *lexer) lexGo(nextState stateFn) stateFn {
	return l.lexGoExtra(nextState, "", "")
}

func (l *lexer) lexGoExtra(nextState stateFn, prefix string, suffix string) stateFn {
	l.acceptRun()
	l.emitRun(prefix, suffix)

	if l.peek() == eof {
		return nil
	}

	if l.isAtCloseTag() {
		if l.openCount <= 0 {
			l.errorf("Close tag with no matching open tag")
			return nil
		}
		l.openCount--
		l.ignoreCloseTag()
		l.emitType(itemEnd)
		l.ignoreNewline()
		return nextState
	}

	// Must be at open tag

	// Allow us to go back into go mode after the next tag is processed
	nextGo := func(l *lexer) stateFn {
		return (*lexer).lexGo(l, nextState)
	}

	return l.lexTag(nextGo)
}

func (l *lexer) lexConvert(nextState stateFn) stateFn {
	l.acceptRun()
	l.emitType(itemRun)

	if l.isAtCloseTag() {
		l.ignoreCloseTag()
		l.emitType(itemEnd)
		return nextState
	}

	nextConvert := func(l *lexer) stateFn {
		return (*lexer).lexConvert(l, nextState)
	}

	return l.lexTag(nextConvert)
}

func (l *lexer) lexText(nextState stateFn) stateFn {
	if !l.isAtCloseTag() {
		l.acceptRun()
		l.emitType(itemRun)
	}

	if l.isAtCloseTag() {
		l.ignoreCloseTag()
		l.emitType(itemEnd)
		return nextState
	}

	if l.peek() == eof {
		if l.blockName != "" {
			return nil
		} else {
			return l.errorf("Looking for close tag, found end of file.")
		}
	}

	nextText := func(l *lexer) stateFn {
		return (*lexer).lexText(l, nextState)
	}

	return l.lexTag(nextText)
}

// isSpace reports whether r is a space character.
func isSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

// isSpace reports whether r is a space character.
func isWhiteSpace(r rune) bool {
	return isSpace(r) || isEndOfLine(r)
}

// isEndOfLine reports whether r is an end-of-line character.
func isEndOfLine(r rune) bool {
	return r == '\r' || r == '\n'
}

// isTagChar reports whether the character is allowed in a tag. Helps us find the end of a tag.
func isTagChar(r rune) bool {
	if r == '}' || isWhiteSpace(r) {
		return false
	} else {
		return true
	}
}

func (l *lexer) isAtOpenTag() bool {
	if len(l.input) < l.pos+2 {
		return false
	}
	return l.input[l.pos:l.pos+2] == "{{"
}

// Test if we are at a close tag. If a close tag is preceeded by a space char, the space char is part of the tag.
func (l *lexer) isAtCloseTag() bool {

	if len(l.input) < l.pos+2 {
		return false // close to eof
	}

	if l.input[l.pos:l.pos+2] == tokEnd {
		return true
	}

	if len(l.input) < l.pos+3 {
		return false // close to eof
	}

	if l.peek() == ' ' && l.input[l.pos+1:l.pos+3] == tokEnd {
		return true
	}

	return false
}

// errorf returns an error token and terminates the scan by passing
// back a nil pointer that will be the next state, terminating l.nextItem.
func (l *lexer) errorf(format string, args ...interface{}) stateFn {
	l.hasError = true
	if l.blockName != "" {
		l.items <- item{typ: itemError, val: "*** Error at line " + strconv.Itoa(l.getLine()) + " of block '" + l.blockName + "': " + fmt.Sprintf(format, args...)}
	} else {
		l.items <- item{typ: itemError, val: "*** Error at line " + strconv.Itoa(l.getLine()) + " of file '" + l.fileName + "': " + fmt.Sprintf(format, args...)}
	}
	return nil
}

// peek returns but does not consume the next rune in the input.
func (l *lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

// accept consumes the next rune if it's from the valid set.
func (l *lexer) accept(valid string) bool {
	if strings.IndexRune(valid, l.next()) >= 0 {
		return true
	}
	l.backup()
	return false
}

// acceptRun consumes a run of runes until it finds an open or close tag
func (l *lexer) acceptRun() {
	var c rune
	for !l.isAtOpenTag() &&
		!l.isAtCloseTag() &&
		c != eof {

		c = l.next()
	}
}

func (l *lexer) acceptUntil(terminators string) {
	for strings.IndexRune(terminators, l.next()) < 0 {
	}
	l.backup()
}

func (l *lexer) acceptTag() string {
	startPos := l.pos
	for {
		r := l.next()
		if !isTagChar(r) {
			l.backup()
			return l.input[startPos:l.pos]
		}
	}
}

func (l *lexer) acceptSpace() {
	for isSpace(l.next()) {
	}
	l.backup()
}

func (l *lexer) getLine() (line int) {
	line = 1
	var pos int
	for pos < l.start {
		c, width := utf8.DecodeRuneInString(l.input[pos:])
		if isEndOfLine(c) {
			line++
		}
		if width == 0 {
			panic("Zero width character found?")
		}
		pos += width
	}
	return
}

func (l *lexer) next() rune {
	var c rune
	if l.pos >= len(l.input) {
		l.width = 0
		return eof
	}
	c, l.width = utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += l.width
	return c
}

func (l *lexer) nextLine() {
	for {
		r := l.next()
		if r == '\n' || r == eof {
			return
		}
	}
}

func (l *lexer) backup() {
	l.pos -= l.width
}

func (l *lexer) ignore() {
	l.start = l.pos
}

func (l *lexer) ignoreRun() {
	l.acceptRun()
	l.ignore()
}

func (l *lexer) ignoreSpace() {
	for {
		r := l.next()
		switch {
		case r == eof:
			return
		case isSpace(r):
			l.ignore()
		default:
			l.backup()
			return
		}
	}
}

func (l *lexer) ignoreWhiteSpace() {
	for {
		r := l.next()
		switch {
		case r == eof:
			return
		case isWhiteSpace(r):
			l.ignore()
		default:
			l.backup()
			return
		}
	}
}

func (l *lexer) ignoreOneSpace() {
	r := l.next()
	switch {
	case r == eof:
		return
	case isWhiteSpace(r):
		l.ignore()
	default:
		l.backup()
		return
	}
}

func (l *lexer) ignoreNewline() {
	r := l.next()
	switch {
	case r == eof:
		return
	case isEndOfLine(r):
		l.ignore()
	default:
		l.backup()
		return
	}
}

func (l *lexer) ignoreCloseTag() {
	if l.isAtCloseTag() {
		r := l.next()
		if r == ' ' {
			l.next() // should be a close tag
		}
		l.next()
		l.ignore()
	}
}

func (l *lexer) currentString() string {
	return l.input[l.start:l.pos]
}
