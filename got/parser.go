package got

import (
	"fmt"
	"html"
	"strings"
	"unicode/utf8"
)

var endedWithNewline = true

type ast struct {
	item  item
	items []ast
}

func Parse(l *lexer) string {
	item := item{typ: itemGo}
	return parseItem(l, item)
}

// Process the template items coming from the lexer and return the whole thing as a string
func parseItem(l *lexer, parent item) string {
	var ret string
	var item item

	for {
		item = l.nextItem()

		if item.typ == itemEOF {
			endedWithNewline = true // prepare for next file
			return ret
		}
		if item.typ == itemEnd || item.typ == itemIgnore {
			return ret
		}

		if item.typ == itemBackup {
			count := "1"
			if item.val != "" {
				count = item.val
			}
			ret += outputTruncate(count)
		} else if item.typ == itemError {
			fmt.Println(item.val)
			return ""
		} else if item.typ == itemRun {
			var out string
			out, endedWithNewline = outputRun(parent, item, endedWithNewline)
			ret += out
		} else {
			ret += parseItem(l, item)
		}
	}
	return ret
}

func outputRun(parent item, item item, prevTextEndedWithNewline bool) (string, bool) {
	switch parent.typ {
	case itemGo:
		return outputGo(item.val, parent.withError), prevTextEndedWithNewline // straight go code

	case itemText: fallthrough
	case itemStrictBlock:
		r,_ := utf8.DecodeLastRuneInString(item.val)
		thisEndedWithNewline := r == '\n'

		if !prevTextEndedWithNewline && item.newline {
			item.val = "\n" + item.val
		}
		return outputText(parent, item.val), thisEndedWithNewline

	case itemConvert:
		return outputHtml(parent, item.val, item.htmlBreaks), false

	default:
		return outputValue(parent, item.val), false

	}
}

func outputGo(code string, withErr bool) string {
	if withErr {
		return fmt.Sprintf(
			"\n{\n err := %s\n"+
				"if err != nil { return err}\n}\n", code)
	} else {
		return code
	}
}

func outputValue(item item, val string) string {
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

	if item.withError {
		return fmt.Sprintf(
			"\n{\nv, err := %s\n"+
				"%s\n"+
				"if err != nil { return err}\n}\n", val,
			fmt.Sprintf(writer, fmt.Sprintf(formatter, "v")))
	} else {
		return fmt.Sprintf("\n%s\n", fmt.Sprintf(writer, fmt.Sprintf(formatter, val)))
	}

}

// outputText sends plain text to the template. There are some nuances here.
// The val includes the space character that comes after the opening tag. We may
// or may not use that character, depending on the circumstances.
func outputText(item item, val string) string {
	if val == "" {
		return ""
	}

	if item.escaped {
		val = html.EscapeString(val)
	}
	if item.translate {
		return "\nt.Translate(" + quoteText(val) + ", buf)\n"
	} else {
		return "\nbuf.WriteString(" + quoteText(val) + ")\n"
	}
}

// Convert text to html
func outputHtml(item item, val string, htmlNewlines bool) string {
	val = html.EscapeString(val)
	if htmlNewlines {
		val = strings.Replace(val, "\r\n", "\n", -1)
		val = strings.Replace(val, "\n\n", "</p><p>", -1)
		val = strings.Replace(val, "\n", "<br>\n", -1)
		val = strings.Replace(val, "</p><p>", "</p>\n<p>", -1) // pretty print it so its inspectable

		val = "<p>" + val + "</p>\n"
	}

	if item.translate {
		return "\nt.Translate(buf, " + quoteText(val) + ")\n"
	} else  {
		return "\nbuf.WriteString(" + quoteText(val) + ")\n"
	}

}

// Generally speaking, text is quoted with a backtick character. However, there is a special case. If the text actually
// contains a backtick character, we cannot use backticks to quote them, but rather double-quotes. This function prepares
// text, looking for these backticks, and then returns a golang quoted text that can be suitably used in all situations.
func quoteText(val string) string {
	return "`" + strings.Replace(val, "`", "` + \"`\" + `", -1) + "`"
}

func outputTruncate(n string) string {
	return fmt.Sprintf("\nbuf.Truncate(buf.Len() - %s)\n", n)
}
