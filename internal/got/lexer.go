package got

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"text/scanner"
)

const eof rune = -1
const errRune rune = -2

type lexer struct {
	fileName  string         // file name being scanned
	blockName string         // named block being scanned
	input     *bufio.Reader  // stream being scanned
	items     chan tokenItem // channel of scanned items
	curBuffer []rune		 // current character run
	lineNum   int			 // line number at start of curBuffer
	lineRuneNum int			 // the char num on the line at start of curBuffer
	backBuffer []rune		 // items that were put back into the lexer after a peek
	relativePaths []string   // when including files, keeps track of the relative paths to search
	namedBlocks map[string]namedBlockEntry
	err error	// most recent error
}

type stateFn func(*lexer) stateFn

// lex opens the file and returns a lexer that will emit tokens on
// the lexer's channel
func lexFile(fileName string, relPaths ...string) *lexer {
	inFile,err := os.Open(fileName)
	defer func() {
		_ = inFile.Close()
	}()

	l := &lexer{
		input:    bufio.NewReader(inFile),
		fileName: fileName,
		items:    make(chan tokenItem),
		relativePaths: relPaths,
	}

	if err != nil {
		l.emitError("error opening file %s", fileName)
		close(l.items)
	} else {
		go l.run()
	}
	return l
}

// lex treats the given string as a block to be inserted
func lexBlock(blockName string, content string) *lexer {
	l := &lexer{
		input:    bufio.NewReader(strings.NewReader(content)),
		blockName: blockName,
		items:    make(chan tokenItem),
	}

	go l.run()
	return l
}


func (l *lexer) run() {
	for state := lexStart; state != nil; {
		state = state(l)
	}
	close(l.items)
}


// Starting state. We start in GO mode.
func lexStart(l *lexer) stateFn {
	return lexRun(l)
}

func lexRun(l *lexer)  stateFn {
	l.ignoreWhiteSpace()
	l.acceptRun()
	l.emitRun()
	if l.isAtCloseTag() {
		l.emitType(itemEnd)
		l.ignoreCloseTag()
		return lexRun
	} else if l.isAtOpenTag() {
		return lexTag
	} else {
		// we are at eof
		return nil
	}
}

func lexParams(l *lexer)  stateFn {
	l.ignoreSpace()

	l.acceptRun()
	if !l.isAtCloseTag() {
		l.emitError("expected closing tag")
		return nil
	}

	paramString := l.currentString()
	params,err := splitParams(paramString)
	if err != nil {
		l.emitError(err.Error())
		return nil
	}
	for _,p := range params {
		l.emit(tokenItem{typ:itemParam, val: p})
	}
	return lexRun
}


// We are pointing to the start of an unknown tag
func lexTag(l *lexer)  stateFn {
	a := l.acceptTag()

	if a == "" {
		panic("expected a tag but no tag found") // a programming error if this happens
	}

	var i tokenItem
	var ok bool

	if i, ok = tokens[a]; !ok {
		// We could not find a known tag, so it could be one of two things:
		// - A custom tag we defined, or
		// - A compressed "interface" tag of the form: {{SomeGoCodeWithValue}}, which will be somewhat similar to the built-in go template engine

		tagName := a[2:]
		if len(tagName)>2 && tagName[len(tagName)-2:] == "}}" {
			tagName = tagName[:len(tagName)-2]
		}
		// if a recognized custom tag, we need to lex it
		if _, ok = l.getNamedBlock(tagName); ok {
			// it's a defined block, so reset to the name of the block
			l.putBackCurBuffer()
			l.next()
			l.next()
			l.ignore()
			i = tokenItem{typ: itemSubstitute}
		} else {
			// we are going to treat it as a go value
			l.putBackCurBuffer()
			l.next()
			l.next()
			l.ignore()
			i = tokenItem{typ: itemInterface, escaped: false, withError: false}
		}
	}

	switch i.typ {
	case itemInclude:
		return l.lexInclude(i.htmlBreaks, i.escaped)

	case itemNamedBlock:
		return l.lexDefineNamedBlock()

	case itemSubstitute:
		return l.lexSubstitute()

	case itemStrictBlock:
		return l.lexStrictBlock()

	case itemComment:
		return l.lexComment()

	case itemText:
		newline := isEndOfLine(l.peek())
		l.ignoreOneSpace()
		i.newline = newline
		l.emit(i)
		return lexRun

	case itemJoin:
		return l.lexJoin()

	default:
		l.emit(i)
		l.ignoreWhiteSpace()
		return lexRun
	}
}

func (l *lexer) emitType(t tokenType) {
	i := tokenItem{typ: t}
	l.emit(i)
}

func (l *lexer) emit(i tokenItem) {
	if i.val == "" {
		i.val = l.currentString()
	}

	i.fileName = l.fileName
	i.blockName = l.blockName
	i.lineNum = l.lineNum
	i.runeNum = l.lineRuneNum

	l.items <- i
	l.ignore()
}

func (l *lexer) emitRun() {
	if l.currentLen() > 0 {
		var i = tokenItem{typ: itemRun}
		l.emit(i)
	}
}

func (l *lexer) lexStrictBlock() stateFn {
	newline := isEndOfLine(l.peek())
	l.ignoreOneSpace()
	l.acceptRun()
	endToken := l.currentString()
	if !l.isAtCloseTag() {
		l.emitError("expected close tag")
		return nil
	}
	l.ignoreCloseTag()
	endToken = "{{end " + endToken + "}}"
	l.acceptUntil(endToken)
	if !l.isAt(endToken) {
		l.emitError("no strict end block found")
		return nil
	}
	l.emit(tokenItem{typ: itemStrictBlock, newline:newline})
	l.ignoreN(len(endToken))
	return lexRun
}

func (l *lexer) lexInclude(htmlBreaks bool, escaped bool) stateFn {
	l.ignore()
	l.acceptRun()
	fileName := strings.TrimSpace(l.currentString())
	if !l.isAtCloseTag() {
		l.emitError("expected close tag")
		return nil // stop
	}
	l.ignoreCloseTag()

	if fileName[0] == '"' {
		var err error
		if fileName, err = strconv.Unquote(fileName); err != nil {
			l.emitError("Include file name error: %s", err.Error())
			return nil // stop
		}
	}

	// Assemble the relative paths collected so far
	var relPath string
	for _,thisPath := range l.relativePaths {
		relPath = filepath.Join(relPath, thisPath)
	}

	// find the file from the include paths, which allows the include paths to override the immediate path
	var foundPath string
	if len(includePaths) > 0 {
		for _, thisPath := range includePaths {
			fileName2 := filepath.Join(thisPath, relPath, fileName)
			if fileExists(fileName2) {
				foundPath = fileName2
			}
		}
	}

	// If not yet found, the find relative to the include file
	if foundPath == "" {
		fileName2 := filepath.Join(filepath.Dir(l.fileName), fileName)
		if fileExists(fileName2) {
			foundPath = fileName2
		}
	}

	if foundPath == "" {
		s := "Could not find include file \"" + fileName + "\""
		s += " in directories "
		if len(includePaths) > 0 {
			s += strings.Join(includePaths, ";") + ":"
		}
		s += filepath.Dir(l.fileName)
		l.emitError(s)
		return nil
	}

	if htmlBreaks || escaped {
		// treat file like a text file
		l.ignore()
		b, err := os.ReadFile(foundPath)
		if err != nil {
			l.emitError("error opening include file %s", foundPath)
			return nil
		}
		l.emit(tokenItem{typ: itemText, escaped: escaped, withError: false, htmlBreaks: htmlBreaks})
		l.emit(tokenItem{typ: itemRun, val: string(b)})
		l.emitType(itemEnd)
		return lexRun
	}

	// lex the include file
	relPaths := make([]string, len(l.relativePaths), len(l.relativePaths) + 1)
	copy(relPaths, l.relativePaths)
	curRelPath := path.Dir(fileName)
	if curRelPath != "" {
		relPaths = append(relPaths, curRelPath)
	}

	l2 := lexFile(foundPath, relPaths...)

	for item := range l2.items {
		l.emit(item) // send items as if they are part of current file
		if item.typ == itemError {
			l.emitError("") // add where the file was included from
			return nil // stop processing
		}
	}

	return lexRun
}


func fileExists(name string) bool {
	_, err := os.Stat(name)
	return 	errors.Is(err, fs.ErrNotExist)
}

func (l *lexer) lexDefineNamedBlock() stateFn {
	l.ignoreSpace()
	l.acceptRun()

	if !l.isAtCloseTag() {
		l.emitError("looking for close tag, found %s", l.peekN(2))
		return nil
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
			l.emitError("item after block name must be the parameter count")
			return nil
		}
	} else if len(items) > 2 {
		l.emitError("block name cannot contain spaces")
		return nil
	}

	if strings.ContainsAny(name, "\t\r\n") {
		l.emitError("block name cannot have tabs or newlines after it")
		return nil
	}

	if _, ok := tokens["{{" + name]; ok {
		l.emitError("block name cannot be a tag name. Block name: %s", name)
		return nil
	}

	endBlock := "{{end " + name + "}}"

	l.acceptUntil(endBlock)
	if !l.isAt(endBlock) {
		l.emitError("no end block found")
	}
	if err := l.addNamedBlock(name, l.currentString(), paramCount); err != nil {
		l.emitError(err.Error())
		return nil
	}
	l.ignoreN(len(endBlock))

	return lexRun
}

func (l *lexer) lexSubstitute() stateFn {
	l.ignoreSpace()
	l.acceptUntil1(" \t}{")
	name := l.currentString()

	if name == "" {
		l.emitError("expected block name, but got empty value")
		return nil
	}

	l.ignoreSpace()
	l.acceptRun()
	paramString := strings.TrimSpace(l.currentString())

	if !l.isAtCloseTag() {
		l.emitError("expected close tag")
		return nil
	}

	l.ignoreCloseTag()

	var block namedBlockEntry
	var ok bool
	var processedBlock string

	if block, ok = l.getNamedBlock(name); !ok {
		l.emitError("named block not found: %s", name)
		return nil
	}

	params, err := splitParams(paramString)
	if err != nil {
		l.emitError(err.Error())
		return nil
	}
	// process parameters
	if processedBlock, err = processParams(name, block, params); err != nil {
		l.emitError(err.Error())
		return nil
	}

	l2 := lexBlock(name, processedBlock)

	for item := range l2.items {
		l.emit(item) // send items as if they are part of current file
		if item.typ == itemError {
			l.emitError("") // add where the file was included from
			return nil // stop processing
		}
	}

	return lexRun
}

func processParams(name string, in namedBlockEntry, params []string) (out string, err error) {
	out = in.text

	var i int
	var s string

	if len(params) == 1 &&
		params[0] == "" &&
		in.paramCount == 0 {
		out = in.text
		return
	}
	if len(params) !=  in.paramCount {
		err = fmt.Errorf("unexpected number of parameters given: expected %d, got %d", in.paramCount, len(params))
		return
	}

	for i, s = range params {
		search := fmt.Sprintf("$%d", i+1)
		out = strings.Replace(out, search, s, -1)
	}

	return
}

// TODO: Test empty params
func splitParams(paramString string) (params []string, err error) {
	var currentItem string

	var s scanner.Scanner
	s.Init(strings.NewReader(paramString))
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		text := s.TokenText()
		if len(text) > 0 && text[0] == '"' && (len(text) == 1 || text[len(text)-1] != '"') {
			err = fmt.Errorf("parameter has a beginning quote with no ending quote: %s", text)
			return
		}
		if text == "," {
			currentItem = strings.TrimSpace(currentItem)
			if currentItem != "" {
				if currentItem[0] == '"' {
					currentItem,err = strconv.Unquote(currentItem)
					if err != nil {
						return
					}
				}
				params = append(params, currentItem)
				currentItem = ""
			} else {
				// insert a blank item
				params = append(params, currentItem)
			}
		} else {
			currentItem += text
		}
	}
	if currentItem != "" {
		if currentItem[0] == '"' {
			currentItem,err = strconv.Unquote(currentItem)
			if err != nil {
				return
			}
		}
		params = append(params, currentItem)
	} else {
		params = append(params, currentItem)
	}

	return
}


func (l *lexer) lexComment() stateFn {
	l.ignoreRun()

	if !l.isAtCloseTag() {
		l.emitError("close tag not found")
		return nil
	}
	l.ignoreCloseTag()
	return lexRun
}


func (l *lexer) lexJoin() stateFn {
	l.emitType(itemJoin)
	return lexParams
}

// emitError emits an error token and terminates the scan by passing
// back a nil pointer that will be the next state, terminating l.nextItem.
func (l *lexer) emitError(format string, args ...interface{}) {
	line,pos := l.calcCurLineNum()
	if l.blockName != "" {
		s := fmt.Sprintf("*** Error at line %d, position %d of block %s: ", line, pos, l.blockName)
		l.items <- tokenItem{typ: itemError, val: s + fmt.Sprintf(format, args...)}
	} else {
		s := fmt.Sprintf("*** Error at line %d, position %d of file %s: ", line, pos, l.fileName)
		l.items <- tokenItem{typ: itemError, val: s + fmt.Sprintf(format, args...)}
	}
}

// isSpace reports whether r is a space character.
func isSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

// isWhiteSpace reports whether r is any kind of white space character.
func isWhiteSpace(r rune) bool {
	return isSpace(r) || isEndOfLine(r) || r == '\r'
}

// isEndOfLine reports whether r is an end-of-line character.
func isEndOfLine(r rune) bool {
	return r == '\n'
}

// isTagChar reports whether the character is allowed in a tag. Helps us find the end of a tag.
func isTagChar(r rune) bool {
	if isWhiteSpace(r) ||
		r == eof ||
		r == errRune ||
		r == '{' {
		return false
	} else {
		return true
	}
}

func (l *lexer) isAtOpenTag() bool {
	return l.isAt(tokBegin)
}

// Test if we are at a close tag. If a close tag is preceded by a space char, the space char is part of the tag.
func (l *lexer) isAtCloseTag() bool {
	return l.isAt(tokEnd) || l.isAt(tokEndWithSpace)
}

func (l *lexer) isAt(pattern string) bool {
	t := l.peekN(len(pattern))
	return t == pattern
}

// peek returns but does not consume the next rune in the input.
func (l *lexer) peek() rune {
	r := l.next()
	if r != eof  && r != errRune {
		l.backup()
	}
	return r
}

// peekN peeks at the next n runes and returns what is found. If eof or an error, the
// string is truncated
func (l *lexer) peekN(n int) (ret string) {
	var i int
	for i = 0; i < n; i++ {
		c := l.next()
		if c == eof || c == errRune {
			break
		}
		ret += string(c)
	}
	for j := 0; j < i; j++ {
		l.backup()
	}
	return
}


// acceptRun consumes a run of runes until it finds an open or close tag, or reaches eof
func (l *lexer) acceptRun() {
	var c rune
	for !l.isAtOpenTag() &&
		!l.isAtCloseTag() &&
		c != eof &&
		c != errRune {

		c = l.next()
	}
}

// acceptUntil will accept runes until it encounters the pattern, or eof or err
func (l *lexer) acceptUntil(pattern string) {
	for {
		s := l.peekN(len(pattern))
		if s == "" {
			// error
			return
		}
		if s == pattern {
			return
		}
		l.next()
	}
}

// acceptUntil1 accepts runes until one of the runes in the terminators string is found
func (l *lexer) acceptUntil1(terminators string) {
	for strings.IndexRune(terminators, l.next()) < 0 {
	}
	l.backup()
}

// acceptTag reads the beginning of a token and returns the token read
func (l *lexer) acceptTag() (ret string) {
	if !l.isAtOpenTag() {
		return ""
	}
	ret += string(l.next())
	ret += string(l.next())
	var foundOne bool

	for {
		r := l.next()
		if r == '}' && foundOne {
			// accept two contiguous closing chars as part of the tag, as long as there is a value
			if l.peek() == '}' {
				l.next()
				ret += "}}"
				return
			} else {
				// likely an error, but reject the closing bracket as part of the tag
				l.backup()
				return
			}
		} else if isTagChar(r) {
			foundOne = true
			ret += string(r)
		} else {
			if r != eof && r != errRune {
				l.backup()
			}
			return ret
		}
	}
}


// next grabs, saves and returns the next rune. All reading of runes from the stream must go through here.
func (l *lexer) next() rune {
	var c rune

	if len(l.backBuffer) > 0 {
		c = l.backBuffer[len(l.backBuffer) - 1]
		l.backBuffer = l.backBuffer[:len(l.backBuffer) - 1]
	} else {
		var err error
		c, _, err = l.input.ReadRune()
		if err != nil {
			if err == io.EOF {
				return eof
			} else {
				l.err = err
				return errRune
			}
		}
	}
	l.curBuffer = append(l.curBuffer, c)
	return c
}

// backup backs up one character. This can happen multiple times.
func (l *lexer) backup() {
	if len(l.curBuffer) == 0 {
		panic("cannot backup here") // this is an error with GoT itself. This should not happen.
	}
	c := l.curBuffer[len(l.curBuffer) - 1]
	l.backBuffer = append(l.backBuffer, c)
	l.curBuffer = l.curBuffer[:len(l.curBuffer) - 1]
}

// putBackCurBuffer puts back the entire curBuffer
func (l *lexer) putBackCurBuffer() {
	for len(l.curBuffer) > 0 {
		l.backup()
	}
}

// ignore empties the current buffer
func (l *lexer) ignore() {
	l.lineNum, l.lineRuneNum = l.calcCurLineNum()
	l.curBuffer = l.curBuffer[:0]
}

func (l *lexer) calcCurLineNum() (lineNum, runeNum int) {
	lineNum = l.lineNum
	runeNum = l.lineRuneNum
	for _, c := range l.curBuffer {
		if isEndOfLine(c) {
			lineNum ++
			runeNum = 0
		} else {
			runeNum ++
		}
	}
	return
}

func (l *lexer) ignoreRun() {
	l.acceptRun()
	l.ignore()
}

// ignoreSpace will advance past spaces and tabs, but not newlines
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

// ignoreWhiteSpace will advance to the next non-whitespace character, ignoring everything read
func (l *lexer) ignoreWhiteSpace() {
	l.ignore()
	for {
		r := l.next()
		switch {
		case r == eof:fallthrough
		case r == errRune:
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
	case r == eof:fallthrough
	case r == errRune:
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
	case r == eof:fallthrough
	case r == errRune:
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

func (l *lexer) ignoreN(n int) {
	for i := 0; i < n; i++ {
		l.next()
	}
	l.ignore()
}




// currentString returns the current buffer as a string
func (l *lexer) currentString() (ret string) {
	for _,c := range l.curBuffer {
		ret += string(c)
	}
	return
}

func (l *lexer) currentLen() int {
	return len(l.curBuffer)
}

func (l *lexer) addNamedBlock (name string, text string, paramCount int) error {
	if l.namedBlocks == nil {
		l.namedBlocks = make(map[string]namedBlockEntry)
	} else if _,ok := l.getNamedBlock(name); ok {
		return fmt.Errorf("named block %s has already been defined", name)
	}
	l.namedBlocks[name] = namedBlockEntry{text, paramCount}
	return nil
}

func (l *lexer) getNamedBlock (name string) (block namedBlockEntry, ok bool) {
	if block, ok = l.namedBlocks[name]; !ok {
		block, ok = includeNamedBlocks[name]
	}
	return
}