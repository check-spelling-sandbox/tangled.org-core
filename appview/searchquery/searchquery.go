package searchquery

import (
	"strings"
	"unicode"
)

type ItemKind int

const (
	KindKeyword ItemKind = iota
	KindQuoted
	KindTagValue
)

type Item struct {
	Kind    ItemKind
	Negated bool
	Raw     string
	Key     string
	Value   string
}

type Query struct {
	items []Item
}

func Parse(input string) *Query {
	q := &Query{}
	runes := []rune(strings.TrimSpace(input))
	if len(runes) == 0 {
		return q
	}

	i := 0
	for i < len(runes) {
		for i < len(runes) && unicode.IsSpace(runes[i]) {
			i++
		}
		if i >= len(runes) {
			break
		}

		negated := false
		if runes[i] == '-' && i+1 < len(runes) && runes[i+1] == '"' {
			negated = true
			i++ // skip '-'
		}

		if runes[i] == '"' {
			start := i
			if negated {
				start-- // include the '-' in Raw
			}
			i++ // skip opening quote
			inner := i
			for i < len(runes) && runes[i] != '"' {
				if runes[i] == '\\' && i+1 < len(runes) {
					i++
				}
				i++
			}
			value := unescapeQuoted(runes[inner:i])
			if i < len(runes) {
				i++ // skip closing quote
			}
			q.items = append(q.items, Item{
				Kind:    KindQuoted,
				Negated: negated,
				Raw:     string(runes[start:i]),
				Value:   value,
			})
			continue
		}

		start := i
		for i < len(runes) && !unicode.IsSpace(runes[i]) && runes[i] != '"' {
			i++
		}
		token := string(runes[start:i])

		negated = false
		subject := token
		if len(subject) > 1 && subject[0] == '-' {
			negated = true
			subject = subject[1:]
		}

		colonIdx := strings.Index(subject, ":")
		if colonIdx > 0 {
			key := subject[:colonIdx]
			value := subject[colonIdx+1:]
			q.items = append(q.items, Item{
				Kind:    KindTagValue,
				Negated: negated,
				Raw:     token,
				Key:     key,
				Value:   value,
			})
		} else {
			q.items = append(q.items, Item{
				Kind:    KindKeyword,
				Negated: negated,
				Raw:     token,
				Value:   subject,
			})
		}
	}

	return q
}

func unescapeQuoted(runes []rune) string {
	var b strings.Builder
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' && i+1 < len(runes) {
			i++
		}
		b.WriteRune(runes[i])
	}
	return b.String()
}

func (q *Query) Items() []Item {
	return q.items
}

func (q *Query) Get(key string) *string {
	for i, item := range q.items {
		if item.Kind == KindTagValue && !item.Negated && item.Key == key {
			return &q.items[i].Value
		}
	}
	return nil
}

func (q *Query) GetAll(key string) []string {
	var result []string
	for _, item := range q.items {
		if item.Kind == KindTagValue && !item.Negated && item.Key == key {
			result = append(result, item.Value)
		}
	}
	return result
}

func (q *Query) GetAllNegated(key string) []string {
	var result []string
	for _, item := range q.items {
		if item.Kind == KindTagValue && item.Negated && item.Key == key {
			result = append(result, item.Value)
		}
	}
	return result
}

func (q *Query) Has(key string) bool {
	return q.Get(key) != nil
}

// KnownTags is the set of tag keys with special system-defined handling.
// Any tag:value pair whose key is not in this set is treated as a dynamic
// label filter.
var KnownTags = map[string]bool{
	"state":  true,
	"author": true,
	"label":  true,
}

// GetDynamicTags returns composite "key:value" strings for all non-negated
// tag:value items whose key is not a known system tag.
func (q *Query) GetDynamicTags() []string {
	var result []string
	for _, item := range q.items {
		if item.Kind == KindTagValue && !item.Negated && !KnownTags[item.Key] {
			result = append(result, item.Key+":"+item.Value)
		}
	}
	return result
}

// GetNegatedDynamicTags returns composite "key:value" strings for all negated
// tag:value items whose key is not a known system tag.
func (q *Query) GetNegatedDynamicTags() []string {
	var result []string
	for _, item := range q.items {
		if item.Kind == KindTagValue && item.Negated && !KnownTags[item.Key] {
			result = append(result, item.Key+":"+item.Value)
		}
	}
	return result
}

func (q *Query) Set(key, value string) {
	raw := key + ":" + value
	found := false
	newItems := make([]Item, 0, len(q.items))

	for _, item := range q.items {
		if item.Kind == KindTagValue && !item.Negated && item.Key == key {
			if !found {
				newItems = append(newItems, Item{
					Kind:  KindTagValue,
					Raw:   raw,
					Key:   key,
					Value: value,
				})
				found = true
			}
		} else {
			newItems = append(newItems, item)
		}
	}

	if !found {
		newItems = append(newItems, Item{
			Kind:  KindTagValue,
			Raw:   raw,
			Key:   key,
			Value: value,
		})
	}

	q.items = newItems
}

func (q *Query) String() string {
	if len(q.items) == 0 {
		return ""
	}

	parts := make([]string, len(q.items))
	for i, item := range q.items {
		parts[i] = item.Raw
	}
	return strings.Join(parts, " ")
}
