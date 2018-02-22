package got

import (
	"html"
	"strings"
	"fmt"
)


type ast struct {
	item item
	items []ast
}

func Parse(l *lexer) string  {
	item := item{typ: itemGo}
	return parseItem(l, item)
}

// Process the template items coming from the lexer and return the whole thing as a string
func parseItem(l *lexer, parent item) string {
	var ret string
	var item item

	for {
		item = l.nextItem()

		if item.typ == itemEOF || item.typ == itemEnd || item.typ == itemIgnore {
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
			ret += outputRun(parent, item)
		} else {
			ret += parseItem(l, item)
		}
	}
	return ret
}


func outputRun(parent item, item item) string {
	var out string

	switch parent.typ {
	case itemGo:
		return outputGo(item.val, parent.withError)	// straight go code

	case itemText:
		/*
		if parent.typ == itemText {
			// itemText within itemText does not make sense, so we treat it like an interface instead. Similar to compressed interface format.
			parent.typ = itemInterface
			return outputValue(parent, item.val)
		}*/
		return outputText(parent, item.val)

	case itemConvert:
		return outputHtml(parent, item.val)

	default:
		return outputValue(parent, item.val)

	}
	return out
}

func outputGo(code string, withErr bool) string {
	if withErr {
		return fmt.Sprintf(
			"\n{\n err := %s\n" +
				"if err != nil { return err}\n}\n", code)
	} else {
		return code
	}
}

func outputValue(item item, val string) string {
	writer := "buf.WriteString(%s)"

	if item.escaped {
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
		formatter = "fmt.Sprintf(\"%%v\", %s)"
	case itemFloat:
		formatter = "strconv.FormatFloat(float64(%s), 'g', -1, 64)"
	case itemBytes:
		formatter = "string(%s[:])"
	default:
		formatter = "%s"
	}

	if item.withError {
		return fmt.Sprintf(
			"\n{\nv, err := %s\n" +
			"%s\n" +
			"if err != nil { return err}\n}\n", val,
			fmt.Sprintf(writer, fmt.Sprintf(formatter, "v")))
	} else {
		return fmt.Sprintf("\n%s\n", fmt.Sprintf(writer, fmt.Sprintf(formatter, val)))
	}

}

func outputText(item item, val string) string {
	if item.escaped {
		val = html.EscapeString(val)
	}
	if item.translate {
		return "\nt.Translate(`" + val + "`, buf)\n"
	} else {
		return "\nbuf.WriteString(`" + val + "`)\n"
	}
}


// Convert text to html
func outputHtml(item item, val string) string {
	val = html.EscapeString(val)
	val = strings.Replace(val, "\n\n", "</p>\n<p>", -1)
	val = strings.Replace(val, "\r\r", "</p>\n<p>", -1)
	val = strings.Replace(val, "\n", "<br>\n", -1)
	val = strings.Replace(val, "\r", "<br>\n", -1)

	val = "<p>" + val + "</p>\n"

	if item.translate {
		return "\nt.Translate(buf, `" + val + "`)\n"
	} else {
		return "\nbuf.WriteString(`" + val + "`)\n"
	}

}

func outputTruncate(n string) string {
	return fmt.Sprintf("\nbuf.Truncate(buf.Len() - %s)\n", n)
}