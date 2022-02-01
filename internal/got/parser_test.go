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

	t.Run("item in item", func(t *testing.T) {
		item := parseContent("{{ a {{ b }} }}")
		assert.Equal(t, itemText, item.childItems[0].typ)
		assert.Equal(t, 3, len(item.childItems[0].childItems))
		assert.Equal(t, "a ", item.childItems[0].childItems[0].val)
		assert.Equal(t, "b ", item.childItems[0].childItems[1].childItems[0].val)
		assert.Equal(t, " ", item.childItems[0].childItems[2].val)
	})


}

func Test_parseErr(t *testing.T) {
	t.Run("go run", func(t *testing.T) {
		item := parseContent("{{")
		assert.Equal(t, itemError, item.typ)
	})

	t.Run("item in item", func(t *testing.T) {
		item := parseContent("{{ a {{ b  }}")
		assert.Equal(t, itemError, item.typ)
	})

	t.Run("missing value 1", func(t *testing.T) {
		item := parseContent("{{i  }}")
		assert.Equal(t, itemError, item.typ)
	})
	t.Run("missing value 2", func(t *testing.T) {
		item := parseContent("{{i }}")
		assert.Equal(t, itemError, item.typ)
	})
	t.Run("missing value 3", func(t *testing.T) {
		item := parseContent("{{ {{i ")
		assert.Equal(t, itemError, item.typ)
	})
	t.Run("missing value 4", func(t *testing.T) {
		item := parseContent("{{ {{i 4")
		assert.Equal(t, itemError, item.typ)
	})

	t.Run("error in value 1", func(t *testing.T) {
		item := parseContent("{{ {{= a{{> abc}} }} }}")
		assert.Equal(t, itemError, item.typ)
	})
	t.Run("error in value 2", func(t *testing.T) {
		item := parseContent("{{ {{= a{{ b }} }} }}")
		assert.Equal(t, itemError, item.typ)
	})
}

func Test_parseIfErr(t *testing.T) {
	t.Run("bad condition 1", func(t *testing.T) {
		item := parseContent("{{if ")
		assert.Equal(t, itemError, item.typ)
	})

	t.Run("bad condition 2", func(t *testing.T) {
		item := parseContent("{{if }}")
		assert.Equal(t, itemError, item.typ)
	})

	t.Run("bad condition 3", func(t *testing.T) {
		item := parseContent("{{if {{> abc}} }}")
		assert.Equal(t, itemError, item.typ)
	})

	t.Run("bad condition 4", func(t *testing.T) {
		item := parseContent("{{if {{= abc}} }}")
		assert.Equal(t, itemError, item.typ)
	})

	t.Run("bad condition 5", func(t *testing.T) {
		item := parseContent("{{if a==b")
		assert.Equal(t, itemError, item.typ)
	})

	t.Run("bad condition 6", func(t *testing.T) {
		item := parseContent("{{if a==b {{> c}}}}d{{if}}")
		assert.Equal(t, itemError, item.typ)
	})

	t.Run("bad condition 7", func(t *testing.T) {
		item := parseContent("{{if a==b {{= c}}}}d{{if}}")
		assert.Equal(t, itemError, item.typ)
	})

	t.Run("bad block 1", func(t *testing.T) {
		item := parseContent("{{if a==b}}d")
		assert.Equal(t, itemError, item.typ)
	})
	t.Run("bad block 2", func(t *testing.T) {
		item := parseContent("{{if a==b}}d{{> d}}{{if}}")
		assert.Equal(t, itemError, item.typ)
	})
	t.Run("bad block 3", func(t *testing.T) {
		item := parseContent("{{if a==b}}d}}")
		assert.Equal(t, itemError, item.typ)
	})

	t.Run("double else", func(t *testing.T) {
		item := parseContent("{{if a==b}}d{{else}}c{{else}}e{{if}}")
		assert.Equal(t, itemError, item.typ)
	})

	t.Run("bad else", func(t *testing.T) {
		item := parseContent("{{if a==b}}d{{else}}")
		assert.Equal(t, itemError, item.typ)
	})

	t.Run("else if after else", func(t *testing.T) {
		item := parseContent("{{if a==b}}d{{else}}c{{elseif g}}h{{if}}")
		assert.Equal(t, itemError, item.typ)
	})

	t.Run("bad else if", func(t *testing.T) {
		item := parseContent("{{if a==b}}c{{elseif ")
		assert.Equal(t, itemError, item.typ)
	})
	t.Run("bad else if 2", func(t *testing.T) {
		item := parseContent("{{if a==b}}c{{elseif }}")
		assert.Equal(t, itemError, item.typ)
	})
	t.Run("bad end block", func(t *testing.T) {
		item := parseContent("{{if  a}}b{{join}}")
		assert.Equal(t, itemError, item.typ)
	})


}

func Test_parseForErr(t *testing.T) {
	t.Run("bad condition 1", func(t *testing.T) {
		item := parseContent("{{for ")
		assert.Equal(t, itemError, item.typ)
	})
	t.Run("bad condition 2", func(t *testing.T) {
		item := parseContent("{{for }}")
		assert.Equal(t, itemError, item.typ)
	})
	t.Run("bad condition 3", func(t *testing.T) {
		item := parseContent("{{for {{> a}}}}")
		assert.Equal(t, itemError, item.typ)
	})
	t.Run("bad condition 4", func(t *testing.T) {
		item := parseContent("{{for {{join}}}}")
		assert.Equal(t, itemError, item.typ)
	})


	t.Run("bad condition 5", func(t *testing.T) {
		item := parseContent("{{for  a{{join}}}}")
		assert.Equal(t, itemError, item.typ)
	})
	t.Run("bad condition 6", func(t *testing.T) {
		item := parseContent("{{for  a{{> b}}}}")
		assert.Equal(t, itemError, item.typ)
	})
	t.Run("bad condition 7", func(t *testing.T) {
		item := parseContent("{{for a")
		assert.Equal(t, itemError, item.typ)
	})


	t.Run("bad block", func(t *testing.T) {
		item := parseContent("{{for  a}}b")
		assert.Equal(t, itemError, item.typ)
	})
	t.Run("bad block 2", func(t *testing.T) {
		item := parseContent("{{for  a}}b{{> c}}{{for}}")
		assert.Equal(t, itemError, item.typ)
	})
	t.Run("bad block 3", func(t *testing.T) {
		item := parseContent("{{for  a}}b}}")
		assert.Equal(t, itemError, item.typ)
	})

	t.Run("bad end block", func(t *testing.T) {
		item := parseContent("{{for  a}}b{{if}}")
		assert.Equal(t, itemError, item.typ)
	})


}

func Test_parseJoinErr(t *testing.T) {
	t.Run("bad params 1", func(t *testing.T) {
		item := parseContent("{{join ")
		assert.Equal(t, itemError, item.typ)
	})

	t.Run("bad params 2", func(t *testing.T) {
		item := parseContent("{{join a}}")
		assert.Equal(t, itemError, item.typ)
	})
	t.Run("bad params 3", func(t *testing.T) {
		item := parseContent("{{join a,b,c}}")
		assert.Equal(t, itemError, item.typ)
	})
	t.Run("bad end", func(t *testing.T) {
		item := parseContent("{{join a,b}}c{{if}}")
		assert.Equal(t, itemError, item.typ)
	})

}
