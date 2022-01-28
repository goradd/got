package got

import (
	"fmt"
	"html"
	"io"
	"os"
	"strings"
)

type  astType struct {
	topItem tokenItem
}

type  astWalker struct {
	w io.Writer
	textMode bool
	escapeText bool
	htmlBreaks bool
	translate bool
}

func buildAst(inFile string) (ret astType, err error) {
	l := lexFile(inFile)
	ret.topItem = parse(l)
	if ret.topItem.typ == itemError {
		return ret, fmt.Errorf(ret.topItem.val)
	}
	return
}

func outputAsts(outPath string, asts ...astType ) error {
	outFile,err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("Could not open output file " + outPath + " error: " + err.Error())
	}
	defer func() {
		_ = outFile.Close()
	}()

	_,err = io.WriteString(outFile, "//** This file was code generated by GoT. DO NOT EDIT. ***\n\n\n")
	if err != nil {
		return fmt.Errorf("Could not write to output file " + outPath + " error: " + err.Error())
	}

	for _,ast := range asts {
		walker := astWalker{w: outFile}
		err = walker.walk(ast.topItem)
		if err != nil {
			break
		}
	}

	return err
}

func (a *astWalker) walk(item tokenItem) error {
	switch item.typ {

	case itemGo:
		defer a.setTextMode(a.textMode, a.escapeText, a.htmlBreaks, a.translate)
		a.setTextMode(false, false, false, false)
		return a.walkItems(item.childItems)

	case itemText:
		defer a.setTextMode(a.textMode, a.escapeText, a.htmlBreaks, a.translate)
		a.setTextMode(true, item.escaped, item.htmlBreaks, item.translate)
		return a.walkItems(item.childItems)

	case itemStrictBlock:
		defer a.setTextMode(a.textMode, a.escapeText, a.htmlBreaks, a.translate)
		a.setTextMode(true, item.escaped, item.htmlBreaks, item.translate)
		return a.outputText(item.val)

	case itemRun:
		return a.outputRun(item)

	case itemString: fallthrough
	case itemBool: fallthrough
	case itemInt: fallthrough
	case itemUInt: fallthrough
	case itemFloat: fallthrough
	case itemInterface: fallthrough
	case itemBytes:
		return a.outputValue(item)

	case itemIf:
		return a.outputIf(item)

	case itemFor:
		return a.outputFor(item)

	case itemJoin:
		return a.outputJoin(item)

	default:
		panic("unexpected token while walking ast")
	}
}

func (a *astWalker) walkItems(items []tokenItem) (err error){
	for _,i := range items {
		err = a.walk(i)
		if err != nil {
			break
		}
	}
	return
}

func (a *astWalker) setTextMode(textMode, escapeText, htmlBreaks bool, translate bool) {
	a.textMode = textMode
	a.escapeText = escapeText
	a.htmlBreaks = htmlBreaks
	a.translate = translate
}

// outputText sends plain text to the template. There are some nuances here.
// The val includes the space character that comes after the opening tag. We may
// or may not use that character, depending on the circumstances.
func (a *astWalker) outputText(val string) (err error) {
	if val == "" {
		return
	}

	if a.escapeText {
		val = html.EscapeString(val)
		if a.htmlBreaks {
			val = strings.Replace(val, "\r\n", "\n", -1)
			val = strings.Replace(val, "\n\n", "</p><p>", -1)
			val = strings.Replace(val, "\n", "<br>\n", -1)
			val = strings.Replace(val, "</p><p>", "</p>\n<p>", -1) // pretty print it so its inspectable

			val = "<p>" + val + "</p>\n"
		}
	}
	if a.translate {
		// translating html quoted text really doesn't make sense, but hey
		_, err = io.WriteString(a.w, "\nt.Translate(" + quoteText(val) + ", buf)\n")

	} else {
		_, err = io.WriteString(a.w, "\nbuf.WriteString(" + quoteText(val) + ")\n")
	}
	return
}


func (a *astWalker) outputRun(item tokenItem) error {
	if !a.textMode {
		return a.outputGo(item.val)
	} else {
		return a.outputText(item.val)
	}
}

func (a *astWalker) outputGo(code string) (err error) {
	_, err = io.WriteString(a.w, code)
	return
}

// outputValue sends a particular value to output
// outputValue overrides the current text environment, but does not change it
func (a *astWalker) outputValue(item tokenItem) (err error) {
	writer := "buf.WriteString(%s)"

	if item.htmlBreaks { // assume escaped too
		writer = `buf.WriteString(strings.Replace(html.EscapeString(%s), "\n", "<br>\n", -1))`
	} else if item.escaped {
		writer = "buf.WriteString(html.EscapeString(%s))"
	}

	var formatter string

	switch item.typ {
	case itemBool:
		formatter = "strconv.FormatBool(%s)"
	case itemInt:
		formatter = "strconv.Itoa(%s)"
	case itemUInt:
		formatter = "strconv.FormatUint(uint64(%s), 10)"
	case itemInterface:
		formatter = "fmt.Sprint(%s)"
	case itemFloat:
		formatter = "strconv.FormatFloat(float64(%s), 'g', -1, 64)"
	case itemBytes:
		formatter = "string(%s[:])"
	default:
		formatter = "%s"
	}

	var out string

	if item.withError {
		out = fmt.Sprintf(
			"\n{\nv, err := %s\n"+
				"%s\n"+
				"if err != nil { return err}\n}\n", item.val,
			fmt.Sprintf(writer, fmt.Sprintf(formatter, "v")))
	} else {
		out = fmt.Sprintf("\n%s\n", fmt.Sprintf(writer, fmt.Sprintf(formatter, item.val)))
	}
	_,err = io.WriteString(a.w, out)
	return
}

// Generally speaking, text is quoted with a backtick character. However, there is a special case. If the text actually
// contains a backtick character, we cannot use backticks to quote them, but rather double-quotes. This function prepares
// text, looking for these backticks, and then returns a golang quoted text that can be suitably used in all situations.
func quoteText(val string) string {
	return "`" + strings.Replace(val, "`", "` + \"`\" + `", -1) + "`"
}

func (a *astWalker) outputIf(item tokenItem) (err error) {
	for _,ifItem := range item.childItems {
		switch ifItem.typ {
		case itemIf:
			_, err = fmt.Fprintf(a.w, "\nif %s {\n", item.val)
		case itemElseIf:
			_,err = fmt.Fprintf(a.w, "\n} else if %s {\n", item.val)
		case itemElse:
			_,err = fmt.Fprintf(a.w, "\n} else {\n")
		}
		if err != nil {return err}

		if err = a.walkItems(item.childItems); err != nil {return err}
		if _,err = fmt.Fprintf(a.w, "\n}\n"); err != nil {return err}
	}
	return
}

func (a *astWalker) outputFor(item tokenItem) (err error) {
	_, err = fmt.Fprintf(a.w, "\nfor %s {\n", item.val)
	if err = a.walkItems(item.childItems); err != nil {return err}
	if _,err = fmt.Fprintf(a.w, "\n}\n"); err != nil {return err}
	return
}

func (a *astWalker) outputJoin(item tokenItem) (err error) {
	_, err = fmt.Fprintf(a.w, `
for _i,_j := range %s {
	_ = _j
`, item.params["slice"].val)
	{
		defer a.setTextMode(a.textMode, a.escapeText, a.htmlBreaks, a.translate)
		a.setTextMode(true, false, false, false)
		if err = a.walkItems(item.childItems); err != nil {return err}
	}
	_,err = fmt.Fprintf(a.w, `
	if _i < len(%s) - 1 {
		buf.WriteString(%q)
	}
}`, item.params["slice"].val, item.params["joinString"].val)
	return
}