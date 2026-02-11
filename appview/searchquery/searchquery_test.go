package searchquery

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseMixed(t *testing.T) {
	q := Parse(`state:open bug "critical issue" label:good-first-issue fix`)
	items := q.Items()
	assert.Equal(t, 5, len(items))

	assert.Equal(t, KindTagValue, items[0].Kind)
	assert.Equal(t, "state", items[0].Key)
	assert.Equal(t, "open", items[0].Value)

	assert.Equal(t, KindKeyword, items[1].Kind)
	assert.Equal(t, "bug", items[1].Raw)

	assert.Equal(t, KindQuoted, items[2].Kind)
	assert.Equal(t, `"critical issue"`, items[2].Raw)

	assert.Equal(t, KindTagValue, items[3].Kind)
	assert.Equal(t, "label", items[3].Key)
	assert.Equal(t, "good-first-issue", items[3].Value)

	assert.Equal(t, KindKeyword, items[4].Kind)

	assert.Equal(t, `state:open bug "critical issue" label:good-first-issue fix`, q.String())
}

func TestGetSetLifecycle(t *testing.T) {
	q := Parse("label:bug state:open keyword label:feature label:urgent")

	// Get returns first match
	val := q.Get("state")
	assert.NotNil(t, val)
	assert.Equal(t, "open", *val)

	// Get returns nil for missing key
	assert.Nil(t, q.Get("author"))

	// Has
	assert.True(t, q.Has("state"))
	assert.False(t, q.Has("author"))

	// GetAll
	assert.Equal(t, []string{"bug", "feature", "urgent"}, q.GetAll("label"))
	assert.Equal(t, 0, len(q.GetAll("missing")))

	// Set updates existing, preserving position
	q.Set("state", "closed")
	assert.Equal(t, "label:bug state:closed keyword label:feature label:urgent", q.String())

	// Set deduplicates
	q.Set("label", "single")
	assert.Equal(t, "label:single state:closed keyword", q.String())

	// Set appends new tag
	q.Set("author", "bob")
	assert.Equal(t, "label:single state:closed keyword author:bob", q.String())
}

func TestParseEmpty(t *testing.T) {
	q := Parse("   ")
	assert.Equal(t, 0, len(q.Items()))
	assert.Equal(t, "", q.String())
}

func TestParseUnclosedQuote(t *testing.T) {
	q := Parse(`"hello world`)
	items := q.Items()
	assert.Equal(t, 1, len(items))
	assert.Equal(t, KindQuoted, items[0].Kind)
	assert.Equal(t, `"hello world`, items[0].Raw)
	assert.Equal(t, "hello world", items[0].Value)
}

func TestParseLeadingColon(t *testing.T) {
	q := Parse(":value")
	items := q.Items()
	assert.Equal(t, 1, len(items))
	assert.Equal(t, KindKeyword, items[0].Kind)
	assert.Equal(t, ":value", items[0].Raw)
}

func TestParseColonInValue(t *testing.T) {
	q := Parse("key:value:with:colons")
	items := q.Items()
	assert.Equal(t, 1, len(items))
	assert.Equal(t, "key", items[0].Key)
	assert.Equal(t, "value:with:colons", items[0].Value)
}

func TestParseEmptyValue(t *testing.T) {
	q := Parse("state:")
	items := q.Items()
	assert.Equal(t, 1, len(items))
	assert.Equal(t, KindTagValue, items[0].Kind)
	assert.Equal(t, "state", items[0].Key)
	assert.Equal(t, "", items[0].Value)
}

func TestQuotedKeyValueIsNotTag(t *testing.T) {
	q := Parse(`"state:open"`)
	items := q.Items()
	assert.Equal(t, 1, len(items))
	assert.Equal(t, KindQuoted, items[0].Kind)
	assert.Equal(t, "state:open", items[0].Value)
	assert.False(t, q.Has("state"))
}

func TestConsecutiveQuotes(t *testing.T) {
	q := Parse(`"one""two"`)
	items := q.Items()
	assert.Equal(t, 2, len(items))
	assert.Equal(t, `"one"`, items[0].Raw)
	assert.Equal(t, `"two"`, items[1].Raw)
}

func TestEscapedQuotes(t *testing.T) {
	q := Parse(`"hello \"world\""`)
	items := q.Items()
	assert.Equal(t, 1, len(items))
	assert.Equal(t, KindQuoted, items[0].Kind)
	assert.Equal(t, `"hello \"world\""`, items[0].Raw)
	assert.Equal(t, `hello "world"`, items[0].Value)
}

func TestEscapedBackslash(t *testing.T) {
	q := Parse(`"hello\\"`)
	items := q.Items()
	assert.Equal(t, 1, len(items))
	assert.Equal(t, KindQuoted, items[0].Kind)
	assert.Equal(t, `hello\`, items[0].Value)
}

func TestNegatedTag(t *testing.T) {
	q := Parse("state:open -label:bug keyword -label:wontfix")
	items := q.Items()
	assert.Equal(t, 4, len(items))

	assert.False(t, items[0].Negated)
	assert.Equal(t, "state", items[0].Key)

	assert.True(t, items[1].Negated)
	assert.Equal(t, KindTagValue, items[1].Kind)
	assert.Equal(t, "label", items[1].Key)
	assert.Equal(t, "bug", items[1].Value)
	assert.Equal(t, "-label:bug", items[1].Raw)

	assert.True(t, items[3].Negated)
	assert.Equal(t, "wontfix", items[3].Value)

	// Get/GetAll/Has skip negated tags
	assert.False(t, q.Has("label"))
	assert.Equal(t, 0, len(q.GetAll("label")))

	// Set doesn't touch negated tags
	q.Set("label", "feature")
	assert.Equal(t, "state:open -label:bug keyword -label:wontfix label:feature", q.String())
}

func TestNegatedBareWordIsKeyword(t *testing.T) {
	q := Parse("-keyword")
	items := q.Items()
	assert.Equal(t, 1, len(items))
	assert.Equal(t, KindKeyword, items[0].Kind)
	assert.Equal(t, "-keyword", items[0].Raw)
}

func TestNegatedQuotedPhrase(t *testing.T) {
	q := Parse(`-"critical bug" state:open`)
	items := q.Items()
	assert.Equal(t, 2, len(items))

	assert.Equal(t, KindQuoted, items[0].Kind)
	assert.True(t, items[0].Negated)
	assert.Equal(t, `-"critical bug"`, items[0].Raw)
	assert.Equal(t, "critical bug", items[0].Value)

	assert.Equal(t, KindTagValue, items[1].Kind)
	assert.Equal(t, "state", items[1].Key)
}

func TestNegatedQuotedPhraseAmongOthers(t *testing.T) {
	q := Parse(`"good phrase" -"bad phrase" keyword`)
	items := q.Items()
	assert.Equal(t, 3, len(items))

	assert.Equal(t, KindQuoted, items[0].Kind)
	assert.False(t, items[0].Negated)
	assert.Equal(t, "good phrase", items[0].Value)

	assert.Equal(t, KindQuoted, items[1].Kind)
	assert.True(t, items[1].Negated)
	assert.Equal(t, "bad phrase", items[1].Value)

	assert.Equal(t, KindKeyword, items[2].Kind)
}

func TestWhitespaceNormalization(t *testing.T) {
	q := Parse("   state:open   keyword   ")
	assert.Equal(t, "state:open keyword", q.String())
}
