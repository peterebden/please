package cmap

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmptyBucketDetection(t *testing.T) {
	// Sanity check: we identify an entry with a zero hash and no key as unused.
	// This works fine as long as our hash function doesn't hash the empty string to zero.
	assert.NotEqual(t, 0, XXHash(""))
}

func TestGetAndSet(t *testing.T) {
	const n = 20
	m := NewOrdered[int](5)
	for i := range n {
		m.Set(strconv.Itoa(i), i)
	}
	for i := range n {
		v, present := m.Get(strconv.Itoa(i))
		assert.True(t, present)
		assert.Equal(t, i, v)
	}
	assert.Equal(t, n, m.Len())
}

func TestIteration(t *testing.T) {
	const n = 22
	m := NewOrdered[int](n / 2)
	for i := range n {
		m.Set(strconv.Itoa(i), i)
	}
	t.Run("Items", func(t *testing.T) {
		x := 0
		for k, v := range m.Items() {
			assert.Equal(t, strconv.Itoa(x), k)
			assert.Equal(t, x, v)
			x++
		}
	})
	t.Run("Keys", func(t *testing.T) {
		x := 0
		for k := range m.Keys() {
			assert.Equal(t, strconv.Itoa(x), k)
			x++
		}
	})
	t.Run("Values", func(t *testing.T) {
		x := 0
		for v := range m.Values() {
			assert.Equal(t, x, v)
			x++
		}
	})

}
