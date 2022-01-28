package got

import "fmt"

type namedBlockEntry struct {
	text string
	paramCount int
}
var namedBlocks = make(map[string]namedBlockEntry)

func addNamedBlock (name string, text string, paramCount int) error {
	if _,ok := namedBlocks[name]; ok {
		return fmt.Errorf("named block %s has already been defined", name)
	}
	namedBlocks[name] = namedBlockEntry{text, paramCount}
	return nil
}

func getNamedBlock(name string) (namedBlockEntry, bool) {
	n,ok := namedBlocks[name]
	return n,ok
}

func duplicateNamedBlocks(in map[string]namedBlockEntry) (newNB map[string]namedBlockEntry) {
	newNB = make(map[string]namedBlockEntry, len(in))
	for k,v := range in {
		newNB[k] = v
	}
	return
}

func useNamedBlocks(in map[string]namedBlockEntry) {
	namedBlocks = duplicateNamedBlocks(in)
}

func getNamedBlocks() map[string]namedBlockEntry {
	return duplicateNamedBlocks(namedBlocks)
}