package got

import (
	"strings"
)

type parser struct {
	lexer *lexer
}

// parse is the main entry point for the recursive parsing process.
//
// parse should return either return a single item that represents the top of the ast tree,
// or an error item that contains the details of what and where the error happened.
func parse(l *lexer) tokenItem {
	p := parser{lexer: l}
	topItem := tokenItem{typ: itemGo}
	var endItem tokenItem
	topItem.childItems, endItem = p.parseRun()

	extraItem := <-p.lexer.items
	if extraItem.typ != itemEOF {
		endItem.typ = itemError // Turn this into an error, we must have too many end tags
	}
	// if we have an error that has no call stack, extract all the errors from the channel and build a call stack from them
	if endItem.typ == itemError {
		if len(endItem.callStack) <= 1 {
			for item := range p.lexer.items {
				if item.typ == itemError {
					endItem.callStack = append(endItem.callStack, item.callStack[0])
				}
			}
		}
		return endItem
	}
	return topItem
}

// parseRun parses a run of text. This is typically text that is between an open and close tag.
func (p *parser) parseRun() (subItems []tokenItem, endItem tokenItem) {
	for item := range p.lexer.items {
		item2 := p.parseRunItem(item)
		switch item2.typ {
		case itemEOF:
			fallthrough
		case itemError:
			fallthrough
		case itemEnd:
			fallthrough
		case itemEndBlock:
			endItem = item2
			return

		default:
			subItems = append(subItems, item2)
		}
	}
	endItem.typ = itemEOF
	return
}

func (p *parser) parseRunItem(item tokenItem) tokenItem {
	switch item.typ {

	// These all do nothing, and eventually just return the item
	case itemEOF:
	case itemError:
	case itemRun:
	case itemStrictBlock:
	case itemEnd:
	case itemEndBlock:

	case itemText:
		fallthrough
	case itemGo:
		var endItem tokenItem
		item.childItems, endItem = p.parseRun()
		if endItem.typ != itemEnd {
			if endItem.typ == itemEOF {
				endItem.typ = itemError
				endItem.val = "unexpected end of file"
			}
			if endItem.typ != itemError {
				endItem.typ = itemError
				endItem.val = "unexpected tag at end of run"
			}
			return endItem
		}
		return item

	case itemString:
		fallthrough
	case itemBool:
		fallthrough
	case itemInt:
		fallthrough
	case itemUInt:
		fallthrough
	case itemFloat:
		fallthrough
	case itemInterface:
		fallthrough
	case itemBytes:
		fallthrough
	case itemGoErr:
		item2 := p.parseValue(item)
		return item2

	case itemIf:
		ifItems := p.parseIf(item)
		if len(ifItems) > 0 {
			if ifItems[0].typ == itemError {
				return ifItems[0]
			}
			// push the if items down to the childItems of overriding if item
			return tokenItem{typ: itemIf, childItems: ifItems}
		}

	case itemJoin:
		item2 := p.parseJoin(item)
		return item2

	case itemFor:
		item2 := p.parseFor(item)
		return item2

	default:
		panic("unexpected token") // this is a programming bug, not a template error
	}

	return item
}

func (p *parser) parseValue(item tokenItem) tokenItem {
	runItem := <-p.lexer.items
	switch runItem.typ {
	case itemRun:
		item.val = strings.TrimSpace(runItem.val)
		if item.val == "" {
			item.typ = itemError
			item.val = "missing value"
			return item
		}
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
		runItem.typ = itemError
		runItem.val = "unexpected text inside a value block"
		return runItem
	}

	endItem := <-p.lexer.items
	switch endItem.typ {
	case itemEnd:
		return item // correctly terminated a value
	case itemEOF:
		endItem.typ = itemError
		endItem.val = "unexpected end of file"
		return endItem
	case itemError:
		return endItem
	default:
		endItem.typ = itemError
		endItem.val = "unexpected text inside a value block"
		return endItem
	}
}

func (p *parser) parseIf(item tokenItem) (items []tokenItem) {
	if item.typ != itemElse {
		conditionItem := <-p.lexer.items
		switch conditionItem.typ {
		case itemRun:
			item.val = strings.TrimSpace(conditionItem.val)
		case itemEnd:
			conditionItem.typ = itemError
			conditionItem.val = "missing condition in if statement"
			return []tokenItem{conditionItem}
		case itemEOF:
			conditionItem.typ = itemError
			conditionItem.val = "unexpected end of file"
			return []tokenItem{conditionItem}
		case itemError:
			return []tokenItem{conditionItem}
		default:
			conditionItem.typ = itemError
			conditionItem.val = "unexpected text inside a value definition"
			return []tokenItem{conditionItem}
		}

		endItem := <-p.lexer.items
		switch endItem.typ {
		case itemEnd:
			// correctly terminated a value, so keep going
		case itemEOF:
			endItem.typ = itemError
			endItem.val = "unexpected end of file"
			return []tokenItem{endItem}
		case itemError:
			return []tokenItem{endItem}
		default:
			endItem.typ = itemError
			endItem.val = "unexpected text inside an if statement"
			return []tokenItem{endItem}
		}
	}

	var endItem tokenItem

	// get the items inside the if statement
	item.childItems, endItem = p.parseRun()

	switch endItem.typ {
	case itemEndBlock:
		// correctly terminated a value, so keep going
	case itemEOF:
		endItem.typ = itemError
		endItem.val = "unexpected end of file"
		return []tokenItem{endItem}
	case itemError:
		return []tokenItem{endItem}
	default:
		endItem.typ = itemError
		endItem.val = "unexpected end of an if statement"
		return []tokenItem{endItem}
	}

	switch endItem.val {
	case "if":
		// terminated the if statement
		return []tokenItem{item}
	case "else":
		if item.typ == itemElse {
			// cannot place an else after an else
			endItem.typ = itemError
			endItem.val = "cannot put an else after another else"
			return []tokenItem{endItem}
		}
		elseItem := endItem
		elseItem.typ = itemElse
		items3 := p.parseIf(elseItem)
		if len(items3) > 0 {
			switch items3[0].typ {
			case itemError:
				return []tokenItem{items3[0]}
			case itemEOF:
				elseItem.typ = itemError
				elseItem.val = "unexpected end of file"
				return []tokenItem{elseItem}
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
		elseIfItem := item
		elseIfItem.typ = itemElseIf
		items3 := p.parseIf(elseIfItem)
		if len(items3) > 0 {
			switch items3[0].typ {
			case itemError:
				return []tokenItem{items3[0]}
			case itemEOF:
				elseIfItem.typ = itemError
				elseIfItem.val = "unexpected end of file"
				return []tokenItem{elseIfItem}
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
		conditionItem.typ = itemError
		conditionItem.val = "missing condition in for statement"
		return conditionItem
	case itemEOF:
		conditionItem.typ = itemError
		conditionItem.val = "unexpected end of file"
		return conditionItem
	case itemError:
		return conditionItem
	default:
		conditionItem.typ = itemError
		conditionItem.val = "unexpected text inside a value definition"
		return conditionItem
	}

	endItem := <-p.lexer.items
	switch endItem.typ {
	case itemEnd:
		// correctly terminated a value, so keep going
	case itemEOF:
		endItem.typ = itemError
		endItem.val = "unexpected end of file"
		return endItem
	case itemError:
		return endItem
	default:
		endItem.typ = itemError
		endItem.val = "unexpected text inside an if statement"
		return endItem
	}

	// get the items inside the for statement
	item.childItems, endItem = p.parseRun()

	switch endItem.typ {
	case itemEndBlock:
		// correctly terminated a value, so keep going
		if endItem.val != "for" {
			endItem.typ = itemError
			endItem.val = "unexpected end block of for, got: " + endItem.val
			return endItem
		}
	case itemEOF:
		endItem.typ = itemError
		endItem.val = "unexpected end of file"
		return endItem
	case itemError:
		return endItem
	default:
		endItem.typ = itemError
		endItem.val = "unexpected end of a for statement"
		return endItem
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
	if endItem.typ != itemEndBlock || endItem.val != "join" {
		endItem.typ = itemError
		endItem.val = "expected ending join tag"
		return endItem
	}
	return item
}
