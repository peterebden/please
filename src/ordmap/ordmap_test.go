package ordmap_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thought-machine/please/src/ordmap"
)

func TestGetAndPut(t *testing.T) {
	var m ordmap.Map[string, string]
	_, present := m.Get("a")
	assert.False(t, present)
	m.Put("a", "b")
	v, present := m.Get("a")
	assert.True(t, present)
	assert.Equal(t, "b", v)
	m.Put("a", "c")
	v, present = m.Get("a")
	assert.True(t, present)
	assert.Equal(t, "c", v)
}

func TestUnion(t *testing.T) {
	var a, b ordmap.Map[string, string]
	a.Put("1", "2")
	a.Put("3", "4")
	a.Put("5", "4")
	a.Put("6", "7")
	b.Put("a", "b")
	b.Put("c", "d")
	b.Put("3", "e")
	c := a.Union(&b)
	assert.Equal(t, []item[string, string]{
		{"1", "2"},
		{"3", "e"},
		{"5", "4"},
		{"6", "7"},
		{"a", "b"},
		{"c", "d"},
	}, Items(c))
}

func TestDelete(t *testing.T) {
	var m ordmap.Map[string, string]
	m.Put("1", "2")
	m.Put("3", "4")
	m.Put("5", "4")
	m.Put("6", "7")
	m.Delete("3")
	actual, _ := m.Get("6")
	assert.Equal(t, "7", actual)
}

// Items returns all the items from a map.
func Items[K comparable, V any](m *ordmap.Map[K, V]) []item[K, V] {
	ret := []item[K, V]{}
	for it := m.Iter(); !it.Done(); it.Next() {
		ret = append(ret, item[K, V]{Key: it.Key(), Val: it.Val()})
	}
	return ret
}

type item[K, V any] struct {
	Key K
	Val V
}
