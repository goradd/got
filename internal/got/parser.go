package got

import (
	"fmt"
	"strings"
)

type parser struct {
	lexer *lexer
}

func parse(l *lexer) tokenItem {
	p := parser{lexer:l}
	topItem := tokenItem{typ: itemGo}
	var endItem tokenItem
	topItem.childItems, endItem = p.parseRun()
	// if we have an error, extract all the errors from the channel and combine them
	if endItem.typ == itemError {
		if endItem.blockName != "" {
			endItem.val = fmt.Sprintf("*** Error at line %d, position %d of block %s: %s", endItem.lineNum, endItem.runeNum, endItem.blockName, endItem.val)
		} else {
			endItem.val = fmt.Sprintf("*** Error at line %d, position %d of file %s: %s", endItem.lineNum, endItem.runeNum, endItem.fileName, endItem.val)
		}

		for item := range p.lexer.items {
			if item.typ == itemError {
				endItem.val += "\n" + item.val
			}
		}
		return endItem
	}
	return topItem
}

func (p *parser) parseRun() (subItems []tokenItem, endItem tokenItem) {
	for item := range p.lexer.items {
		switch item.typ {
		case itemEOF:
			endItem = item
			return

		case itemError:
			endItem = item
			return

		case itemRun:
			subItems = append(subItems, item)

		case itemStrictBlock:
			subItems = append(subItems, item)

		case itemEnd:
			endItem = item
			return

		case itemEndBlock:
			endItem = item
			return

		case itemText:fallthrough
		case itemGo:
			item.childItems, endItem = p.parseRun()
			if endItem.typ != itemEnd {
				return
			}
			subItems = append(subItems, item)

		case itemString: fallthrough
		case itemBool: fallthrough
		case itemInt: fallthrough
		case itemUInt: fallthrough
		case itemFloat: fallthrough
		case itemInterface: fallthrough
		case itemBytes:
			item2 := p.parseValue(item)
			if item2.typ == itemError {
				endItem = item2
				return
			}
			subItems = append(subItems, item2)

		case itemIf:
			ifItems := p.parseIf(item)
			if len(ifItems) > 0 {
				if ifItems[0].typ == itemError {
					endItem = ifItems[0]
					return
				}
				// push the if items down to the childItems of overriding if item
				ifItem := tokenItem{typ: itemIf, childItems: ifItems}
				subItems = append(subItems, ifItem)
			}

		case itemJoin:
			item2 := p.parseJoin(item)
			if item2.typ == itemError {
				endItem = item2
				return
			}
			subItems = append(subItems, item2)

		case itemFor:
			item2 := p.parseFor(item)
			if item2.typ == itemError {
				endItem = item2
				return
			}
			subItems = append(subItems, item2)

		default:
			panic("unexpected item " + item.typ.String()) // this is a programming bug, not a template error
		}
	}
	return
}

func (p *parser) parseValue(item tokenItem) tokenItem {
	runItem := <- p.lexer.items
	switch runItem.typ {
	case itemRun:
		item.val = strings.TrimSpace(runItem.val)
	case itemEnd:
		item.typ = itemError
		item.val = "missing value"
		return item
	case itemEOF:
		item.typ = itemError
		item.val = "unexpected end of file"
		return item
	case itemError:
		return runItem
	default:
		panic("unexpected item inside a value block") // this is programming but, not a template error
	}

	endItem := <- p.lexer.items
	switch endItem.typ {
	case itemEnd:
	return item // correctly terminated a value
	case itemEOF:
		item.typ = itemError
		item.val = "unexpected end of file"
		return item
	case itemError:
		return endItem
	default:
		item.typ = itemError
		item.val = "unexpected text inside a value definition"
		return item
	}
}

func (p *parser) parseIf(item tokenItem) (items []tokenItem) {
	if item.typ != itemElse {
		conditionItem := <-p.lexer.items
		switch conditionItem.typ {
		case itemRun:
			item.val = strings.TrimSpace(conditionItem.val)
		case itemEnd:
			item.typ = itemError
			item.val = "missing condition in if statement"
			return []tokenItem{item}
		case itemEOF:
			item.typ = itemError
			item.val = "unexpected end of file"
			return []tokenItem{item}
		case itemError:
			return []tokenItem{conditionItem}
		default:
			item.typ = itemError
			item.val = "unexpected text inside a value definition"
			return []tokenItem{item}
		}

		endItem := <-p.lexer.items
		switch endItem.typ {
		case itemEnd:
			// correctly terminated a value, so keep going
		case itemEOF:
			item.typ = itemError
			item.val = "unexpected end of file"
			return []tokenItem{item}
		case itemError:
			return []tokenItem{endItem}
		default:
			item.typ = itemError
			item.val = "unexpected text inside an if statement"
			return []tokenItem{item}
		}
	}

	var endItem tokenItem

	// get the items inside the if statement
	item.childItems, endItem = p.parseRun()

	switch endItem.typ {
	case itemEndBlock:
		// correctly terminated a value, so keep going
	case itemEOF:
		item.typ = itemError
		item.val = "unexpected end of file"
		return []tokenItem{item}
	case itemError:
		return []tokenItem{endItem}
	default:
		item.typ = itemError
		item.val = "unexpected text inside an if statement"
		return []tokenItem{item}
	}

	switch endItem.val {
	case "if":
		// terminated the if statement
		return []tokenItem{item}
	case "else":
		if item.typ == itemElse {
			// cannot place an else after an else
			item.typ = itemError
			item.val = "cannot put an else after another else"
			return []tokenItem{item}
		}
		elseItem := tokenItem{typ: itemElse}
		items3 := p.parseIf(elseItem)
		if len(items3) > 0 {
			switch items3[0].typ {
			case itemError:
				return []tokenItem{items3[0]}
			case itemEOF:
				item.typ = itemError
				item.val = "unexpected end of file"
				return []tokenItem{item}
			}
		}
		items = append(items, item)
		items = append(items, items3...)
		return

	case "elseif":
		if item.typ == itemElse {
			// cannot place an else after an else
			item.typ = itemError
			item.val = "cannot put an elseif after an else"
			return []tokenItem{item}
		}
		elseIfItem := tokenItem{typ: itemElseIf}
		items3 := p.parseIf(elseIfItem)
		if len(items3) > 0 {
			switch items3[0].typ {
			case itemError:
				return []tokenItem{items3[0]}
			case itemEOF:
				item.typ = itemError
				item.val = "unexpected end of file"
				return []tokenItem{item}
			}
		}
		items = append(items, item)
		items = append(items, items3...)
		return

	default:
		item.typ = itemError
		item.val = "unexpected end block item"
		return []tokenItem{item}
	}
}
func (p *parser) parseFor(item tokenItem) tokenItem {
	conditionItem := <-p.lexer.items
	switch conditionItem.typ {
	case itemRun:
		item.val = strings.TrimSpace(conditionItem.val)
	case itemEnd:
		item.typ = itemError
		item.val = "missing condition in for statement"
		return item
	case itemEOF:
		item.typ = itemError
		item.val = "unexpected end of file"
		return item
	case itemError:
		return conditionItem
	default:
		item.typ = itemError
		item.val = "unexpected text inside a value definition"
		return item
	}

	endItem := <-p.lexer.items
	switch endItem.typ {
	case itemEnd:
		// correctly terminated a value, so keep going
	case itemEOF:
		item.typ = itemError
		item.val = "unexpected end of file"
		return item
	case itemError:
		return endItem
	default:
		item.typ = itemError
		item.val = "unexpected text inside an if statement"
		return item
	}

	// get the items inside the for statement
	item.childItems, endItem = p.parseRun()

	switch endItem.typ {
	case itemEndBlock:
		// correctly terminated a value, so keep going
	case itemEOF:
		item.typ = itemError
		item.val = "unexpected end of file"
		return item
	case itemError:
		return endItem
	default:
		item.typ = itemError
		item.val = "unexpected text inside a for statement"
		return item
	}
	return item
}

func (p *parser) parseJoin(item tokenItem) tokenItem {
	sliceItem := <-p.lexer.items
	connectorItem := <-p.lexer.items

	if sliceItem.typ != itemParam || connectorItem.typ != itemParam {
		item.typ = itemError
		item.val = "expected parameter of join statement"
		return item
	}
	item.params = make(map[string]tokenItem)
	item.params["slice"] = sliceItem
	item.params["joinString"] = connectorItem
	endItem := <-p.lexer.items
	if endItem.typ != itemEnd {
		endItem.typ = itemError
		endItem.val = "expected end of join statement"
		return endItem
	}
	item.childItems, endItem = p.parseRun()
	if endItem.typ != itemEndBlock  || endItem.val != "join"{
		endItem.typ = itemError
		endItem.val = "expected ending join tag"
		return endItem
	}
	return item
}

