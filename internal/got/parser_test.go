package got

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func parseContent(content string) tokenItem {
	l := lexBlock("test", content, nil)
	i := parse(l)
	return i
}

func Test_parse(t *testing.T) {
	t.Run("go run", func(t *testing.T) {
		item := parseContent("abc")
		assert.Equal(t, itemGo, item.typ)
		assert.Equal(t, 1, len(item.childItems))
		assert.Equal(t, itemRun, item.childItems[0].typ)
	})
	t.Run("text run", func(t *testing.T) {
		item := parseContent("{{ abc }}")
		assert.Equal(t, itemText, item.childItems[0].typ)
		assert.Equal(t, 1, len(item.childItems[0].childItems))
		assert.Equal(t, itemRun, item.childItems[0].childItems[0].typ)
	})

	t.Run("strict block", func(t *testing.T) {
		item := parseContent("{{begin a}} abc {{end a}}")
		assert.Equal(t, itemStrictBlock, item.childItems[0].typ)
		assert.Equal(t, " abc ", item.childItems[0].val)
	})

	t.Run("value", func(t *testing.T) {
		item := parseContent("{{= a }}")
		assert.Equal(t, itemString, item.childItems[0].typ)
		assert.Equal(t, "a", item.childItems[0].val)
	})

	t.Run("value with error", func(t *testing.T) {
		item := parseContent("{{ie b }}")
		assert.Equal(t, itemInt, item.childItems[0].typ)
		assert.True(t, item.childItems[0].withError)
		assert.False(t, item.childItems[0].escaped)
		assert.Equal(t, "b", item.childItems[0].val)
	})

	t.Run("if", func(t *testing.T) {
		item := parseContent("{{if a<b}}c{{if}}")
		assert.Equal(t, itemIf, item.childItems[0].typ)
		assert.Equal(t, itemIf, item.childItems[0].childItems[0].typ)
		assert.Equal(t, "a<b", item.childItems[0].childItems[0].val)
		assert.Equal(t, itemRun, item.childItems[0].childItems[0].childItems[0].typ)
		assert.Equal(t, "c", item.childItems[0].childItems[0].childItems[0].val)
	})
	t.Run("elseif", func(t *testing.T) {
		item := parseContent("{{if a<b}}c{{elseif d<e}}d{{else}}e{{if}}")
		assert.Equal(t, itemIf, item.childItems[0].typ)
		assert.Equal(t, itemIf, item.childItems[0].childItems[0].typ)
		assert.Equal(t, "a<b", item.childItems[0].childItems[0].val)
		assert.Equal(t, itemElseIf, item.childItems[0].childItems[1].typ)
		assert.Equal(t, "d<e", item.childItems[0].childItems[1].val)
		assert.Equal(t, itemRun, item.childItems[0].childItems[1].childItems[0].typ)
		assert.Equal(t, "d", item.childItems[0].childItems[1].childItems[0].val)
		assert.Equal(t, itemElse, item.childItems[0].childItems[2].typ)
		assert.Equal(t, itemRun, item.childItems[0].childItems[2].childItems[0].typ)
		assert.Equal(t, "e", item.childItems[0].childItems[2].childItems[0].val)
		assert.Equal(t, itemRun, item.childItems[0].childItems[0].childItems[0].typ)
		assert.Equal(t, "c", item.childItems[0].childItems[0].childItems[0].val)
	})

	t.Run("for", func(t *testing.T) {
		item := parseContent("{{for a := range b}}c{{for}}")
		assert.Equal(t, itemFor, item.childItems[0].typ)
		assert.Equal(t, "a := range b", item.childItems[0].val)
		assert.Equal(t, itemRun, item.childItems[0].childItems[0].typ)
		assert.Equal(t, "c", item.childItems[0].childItems[0].val)
	})

	t.Run("join", func(t *testing.T) {
		item := parseContent("{{join a.b, c}}d{{join}}")
		assert.Equal(t, itemJoin, item.childItems[0].typ)
		assert.Equal(t, 2, len(item.childItems[0].params))
		assert.Equal(t, "a.b", item.childItems[0].params["slice"].val)
		assert.Equal(t, "c", item.childItems[0].params["joinString"].val)
		assert.Equal(t, 1, len(item.childItems[0].childItems))
		assert.Equal(t, itemRun, item.childItems[0].childItems[0].typ)
		assert.Equal(t, "d", item.childItems[0].childItems[0].val)
	})

	t.Run("join2", func(t *testing.T) {
		item := parseContent("{{join a.b, c}}{{i d}}{{join}}")
		assert.Equal(t, itemJoin, item.childItems[0].typ)
		assert.Equal(t, 2, len(item.childItems[0].params))
		assert.Equal(t, "a.b", item.childItems[0].params["slice"].val)
		assert.Equal(t, "c", item.childItems[0].params["joinString"].val)
		assert.Equal(t, 1, len(item.childItems[0].childItems))
		assert.Equal(t, itemInt, item.childItems[0].childItems[0].typ)
		assert.Equal(t, "d", item.childItems[0].childItems[0].val)
	})

}
