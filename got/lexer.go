package got

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"text/scanner"
	"time"
	"unicode/utf8"
)

const eof = -1

type Pos int

var IncludePaths []string
var IncludeFiles []string

type namedBlockEntry struct {
	text string
	paramCount int
}
var namedBlocks map[string]namedBlockEntry

func init() {
	namedBlocks = make(map[string]namedBlockEntry)
}

func (i tokenItem) String() string {
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
	fileName  string         // file name being scanned
	blockName string         // named block being scanned
	input     string         // string being scanned
	start     int            // start position of tokenItem
	pos       int            // current position
	width     int            // width of last rune
	items     chan tokenItem // channel of scanned items
	hasError  bool
	openCount int // Make sure open and close tags are matched
	relativePaths []string // when including files, keeps track of the relative paths to search
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
func (l *lexer) nextItem() tokenItem {

	select {
	case i := <-l.items:
		//l.lastPos = l.pos
		return i
	case <-time.After(10 * time.Second): // Internal error? We are supposed to detect EOF situations before we get here
		//close(l.items)
		return tokenItem{typ: itemError, val: "*** Internal error at line " + strconv.Itoa(l.getLine()) + " read past end of file " + l.fileName + ". Are you missing an end tag?"}
	}
}

func Lex(input string, fileName string) *lexer {
	l := &lexer{
		input:    input,
		fileName: fileName,
		items:    make(chan tokenItem),
	}
	// make predefined named blocks
	filePath, err := filepath.Abs(fileName)
	base := filepath.Base(filePath)
	root := base
	if offset := strings.Index(root, "."); offset >= 0 {
		root = root[0:offset]
	}
	dir := filepath.Base(filepath.Dir(filePath))

	if err == nil {
		namedBlocks["templatePath"] = namedBlockEntry{filePath, 0}
		namedBlocks["templateName"] = namedBlockEntry{base, 0}
		namedBlocks["templateRoot"] = namedBlockEntry{root, 0}
		namedBlocks["templateDir"] = namedBlockEntry{dir, 0}
	}

	go l.run()
	return l
}

func (l *lexer) emitType(t tokenType) {
	i := tokenItem{typ: t}
	l.emit(i)
}

func (l *lexer) emit(i tokenItem) {
	if i.val == "" {
		i.val = l.input[l.start:l.pos]
	}

	l.items <- i
	l.start = l.pos
	//fmt.Printf("%v", tokenItem)

}

func (l *lexer) emitRun(prefix string, suffix string) {
	var i = tokenItem{typ: itemRun, val: prefix + l.input[l.start:l.pos] + suffix}
	l.emit(i)
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

	var i tokenItem
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
		newline := isEndOfLine(l.peek())
		l.ignoreOneSpace()
		return l.lexStrictBlock(priorState, newline)

	case itemInclude:
		return l.lexInclude(priorState, i.htmlBreaks, i.escaped)

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
		newline := isEndOfLine(l.peek())
		l.ignoreOneSpace()
		return l.lexText(priorState, newline)

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

func (l *lexer) lexStrictBlock(nextState stateFn, newline bool) stateFn {
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
	l.emit(tokenItem{typ: itemRun, newline:newline})
	l.start += len(endToken) // skip end token
	l.pos = l.start
	l.width = 0
	l.emitType(itemEnd)
	return nextState
}

func (l *lexer) lexInclude(nextState stateFn, htmlBreaks bool, escaped bool) stateFn {
	l.ignore()
	l.acceptRun()
	fileName := strings.TrimSpace(l.currentString())
	if !l.isAtCloseTag() {
		return l.errorf("Expected close tag")
	}
	l.ignoreCloseTag()

	var err error
	if fileName[0] == '"' {
		if fileName, err = strconv.Unquote(fileName); err != nil {
			return l.errorf("Include file name error: %s", err.Error())
		}
	}

	// Assemble the relative paths collected so far
	var relPath string
	for _,thisPath := range l.relativePaths {
		relPath = filepath.Join(relPath, thisPath)
	}

	curRelPath := path.Dir(fileName)

	// find the file from the include paths, which allows the include paths to override the immediate path
	var buf []byte
	if len(IncludePaths) > 0 {
		for _, thisPath := range IncludePaths {
			fileName2 := filepath.Join(thisPath, relPath, fileName)
			if buf, err = ioutil.ReadFile(fileName2); err == nil {
				log.Println("Opened " + fileName2)
				fileName = fileName2
				break
			}
			if !os.IsNotExist(err) {
				return l.errorf("File read error: %s", err.Error())
			}
		}
	}

	// If not yet found, the find relative to the include file
	if buf == nil || len(buf) == 0 {
		fileName2 := filepath.Join(filepath.Dir(l.fileName), fileName)
		buf, err = ioutil.ReadFile(fileName2)
		if err == nil {
			fileName = fileName2
		}
	}

	if os.IsNotExist(err) {
		s := "Could not find include file \"" + fileName + "\""
		s += " in directories "
		if len(IncludePaths) > 0 {
			s += strings.Join(IncludePaths, ";") + ":"
		}
		s += filepath.Dir(l.fileName)
		return l.errorf(s)
	}
	if err != nil {
		return l.errorf("File read error: %s", err.Error())
	}

	s := string(buf[:])

	if htmlBreaks || escaped {
		// treat file like a text file
		l.ignore()
		l.emitType(itemConvert)
		l.emit(tokenItem{typ: itemRun, val: s, htmlBreaks: htmlBreaks, escaped: escaped})
		l.emitType(itemEnd)

		return nextState
	}

	// lex the include file

	l2 := &lexer{
		input:    s,
		fileName: fileName,
		items:    l.items,
		relativePaths: l.relativePaths,
	}
	if curRelPath != "" {
		l2.relativePaths = append(l2.relativePaths, curRelPath)
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

	items := strings.Split(name," ")
	var paramCount int
	if len(items) == 2 {
		var err error
		name = items[0]
		paramCount,err = strconv.Atoi(items[1])
		if err != nil {
			return l.errorf("Item after block name must be the parameter count")
		}
	} else if len(items) > 2 {
		return l.errorf("Block name cannot contain spaces")
	}

	if strings.ContainsAny(name, "\t\r\n") {
		return l.errorf("Block name cannot have tabs or newlines after it")
	}

	if _, ok := tokens["{{"+name]; ok {
		return l.errorf("Block name cannot be a tag name. Block name: %s", name)
	}

	offset := strings.Index(l.input[l.start:], tokEndBlock)
	if offset == -1 {
		return l.errorf("No end block found")
	}

	namedBlocks[name] = namedBlockEntry{l.input[l.start : l.start+offset], paramCount}
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

	var block namedBlockEntry
	var ok bool
	var processedBlock string

	if block, ok = namedBlocks[name]; !ok {
		return l.errorf("Named block not found: %s", name)
	}

	// process parameters
	var err error
	if processedBlock, err = processParams(name, block, paramString); err != nil {
			return l.errorf(err.Error())
		}

	l2 := &lexer{
		input:     processedBlock,
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

func processParams(name string, in namedBlockEntry, paramString string) (out string, err error) {
	paramString = strings.TrimSpace(paramString)
	params, err := splitParams(paramString)

	if err != nil {
		return
	}
	out = in.text

	var i int
	var s string
	for i, s = range params {
		if i >= in.paramCount {
			err = fmt.Errorf("too many parameters given for named block %s", name)
		}
		search := fmt.Sprintf("$%d", i+1)
		out = strings.Replace(out, search, s, -1)
	}

	for ; i < in.paramCount; i++ {
		// missing parameters will get an empty value
		search := fmt.Sprintf("$%d", i+1)
		out = strings.Replace(out, search, "", -1)
	}

	return
}

func splitParams(paramString string) (params []string, err error) {
	var currentItem string

	var s scanner.Scanner
	s.Init(strings.NewReader(paramString))
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		text := s.TokenText()
		if len(text) > 0 && text[0] == '"' && (len(text) == 1 || text[len(text)-1] != '"') {
			err = fmt.Errorf("defined block parameter has a beginning quote with no ending quote: %s", text)
			return
		}
		if text == "," {
			currentItem = strings.TrimSpace(currentItem)
			if currentItem != "" {
				if currentItem[0] == '"' {
					if len(currentItem) > 1 && currentItem[len(currentItem) - 1] == '"' {
						//currentItem = currentItem[1 : len(currentItem)-1]
						currentItem,err = strconv.Unquote(currentItem)
					} else {
						err = fmt.Errorf("defined block parameter starts with a quote but does not end with a quote: %s", currentItem)
						return
					}
				}
				params = append(params, currentItem)
				currentItem = ""
			}
		} else {
			currentItem += text
		}
	}
	if currentItem != "" {
		if currentItem[0] == '"' {

			if len(currentItem) > 1 && currentItem[len(currentItem) - 1] == '"' {
				//currentItem = currentItem[1 : len(currentItem)-1]
				currentItem,err = strconv.Unquote(currentItem)
			} else {
				err = fmt.Errorf("defined block parameter starts with a quote but does not end with a quote: %s", currentItem)
				return
			}
		}
		params = append(params, currentItem)
	}
	return
/*
	items := strings.Split(paramString, ",")

	// Check to see if we split something surrounded by quotes
	for _, tokenItem := range items {
		cleanItem = strings.TrimSpace(tokenItem)
		if len(cleanItem) == 0 {
			if currentItem != "" {
				currentItem += "," + tokenItem
			} else {
				params = append(params, cleanItem)
			}
		} else if cleanItem[0:1] == "\"" {
			if cleanItem[len(cleanItem)-1:] == "\"" {
				if len(cleanItem) > 1 && tokenItem[len(cleanItem)-2:len(cleanItem)-1] != "\\" {
					// an tokenItem bounded by quotes, so add it without the quotes
					currentItem = tokenItem[1 : len(cleanItem)-1]
					currentItem = cleanEscapedQuotes(currentItem)
					params = append(params, currentItem)
					currentItem = ""
				} else if len(cleanItem) == 1 {
					// a single quote, so either begin or end with a blank tokenItem
					if currentItem == "" {
						// start the tokenItem
						currentItem = ","
					} else {
						currentItem += ","
						params = append(params, currentItem)
						currentItem = ""
					}
				} else {
					// an tokenItem started with a quote, but ended with an escaped quote, so build the string using the original non-cleaned tokenItem.
					offset := strings.Index(tokenItem, "\"")
					currentItem = cleanEscapedQuotes(tokenItem[offset+1:])
				}
			} else {
				// an tokenItem started with a quote, but not ended with a quote, so build the tokenItem
				offset := strings.Index(tokenItem, "\"")
				currentItem = cleanEscapedQuotes(tokenItem[offset+1:])
			}
		} else {
			if cleanItem[len(cleanItem)-1:] == "\"" {
				if len(cleanItem) > 1 && cleanItem[len(cleanItem)-2:len(cleanItem)-1] != "\\" {
					// an tokenItem ending with a quote, but not started with a quote
					if currentItem != "" {
						lastOffset := strings.LastIndex(tokenItem, "\"")
						currentItem += "," + cleanEscapedQuotes(tokenItem[:lastOffset])
						params = append(params, currentItem)
						currentItem = ""
					} else {
						err = fmt.Errorf("Defined block parameter ends with a quote but does not start with a quote: %s", tokenItem)
						return
					}
				} else {
					// an tokenItem ending with an escaped quote, so just include it.
					if currentItem != "" {
						currentItem += "," + cleanEscapedQuotes(tokenItem)
					} else {
						params = append(params, cleanItem)
					}
				}
			} else {
				// A normal tokenItem
				if currentItem != "" {
					currentItem += "," + cleanEscapedQuotes(tokenItem)
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
/*
func cleanEscapedQuotes(s string) string {
	return strings.Replace(s, "\\\"", "\"", -1)
}*/

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
	if l.isAtCloseTag() { // a plain else
		return l.lexGoExtra(nextState, " } else {\n", "")
	} else { // an else if?
		l.ignoreWhiteSpace()
		l.acceptUntil(" ")
		if l.currentString() != "if" {
			return l.errorf("An 'else' can only be followed by an 'if' or nothing, found %s", l.currentString())
		}
		return l.lexGoExtra(nextState, " } else ", " { ")
	}
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

func (l *lexer) lexText(nextState stateFn, newline bool) stateFn {

	if !l.isAtCloseTag() {
		l.acceptRun()
		l.emit(tokenItem{typ: itemRun, newline:newline})
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
		return (*lexer).lexText(l, nextState, false)
	}

	return l.lexTag(nextText)
}

// isSpace reports whether r is a space character.
func isSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

// isWhiteSpace reports whether r is any kind of white space character.
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

	// This is the way to prevent generating a newline at the end of text
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
		l.items <- tokenItem{typ: itemError, val: "*** Error at line " + strconv.Itoa(l.getLine()) + " of block '" + l.blockName + "': " + fmt.Sprintf(format, args...)}
	} else {
		l.items <- tokenItem{typ: itemError, val: "*** Error at line " + strconv.Itoa(l.getLine()) + " of file '" + l.fileName + "': " + fmt.Sprintf(format, args...)}
	}
	return nil
}

// peek returns but does not consume the next rune in the input.
func (l *lexer) peek() rune {
	r := l.next()
	if r != eof {
		l.backup()
	}
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
	if !isTagChar(l.peek()) {
		panic("Accept tag is not at the start of a tag")
	}
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
	l.ignore()
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
	l.ignore()
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

// ignoreOneSpace ignores one space, INCLUDING white space characters.
func (l *lexer) ignoreOneSpace() {
	l.ignore()
	r := l.next()
	switch {
	case r == eof:
		return
	case r == '\r':
		r = l.next()
		if r == '\n' {
			l.ignore()
		} else {
			l.backup()
		}
		return
	case r == '\n':
		l.ignore()
		return
	case isSpace(r):
		l.ignore()
	default:
		l.backup()
		return
	}
}

// ignoreNewline steps over a newline and ignores it. If we are not on a newline, nothing will happen.
func (l *lexer) ignoreNewline() {
	l.ignore()
	r := l.next()
	switch {
	case r == eof:
		return
	case r == '\r':
		r = l.next()
		if r == '\n' {
			l.ignore()
		} else {
			l.backup()
		}
		return
	case r == '\n':
		l.ignore()
		return
	default:
		l.backup()
		return
	}
}

func (l *lexer) ignoreCloseTag() {
	l.ignore()
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
