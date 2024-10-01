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

func TestCopy(t *testing.T) {
	const n = 20
	m := NewOrdered[int](5)
	for i := range n {
		m.Set(strconv.Itoa(i), i)
	}
	m2 := m.Copy()
	assert.Equal(t, n, m2.Len())
	for i := range n {
		v, present := m2.Get(strconv.Itoa(i))
		assert.True(t, present)
		assert.Equal(t, i, v)
	}
}

func TestUnion(t *testing.T) {
	m1 := NewOrdered[int](0)
	m2 := NewOrdered[int](0)
	m1.Set("1", 1)
	m1.Set("3", 3)
	m2.Set("1", 2)
	m2.Set("4", 4)
	t.Run("Left to right", func(t *testing.T) {
		n := m1.Union(m2)
		assert.Equal(t, 3, n.Len())
		v, _ := n.Get("1")
		assert.Equal(t, 2, v)
	})
	t.Run("Right to left", func(t *testing.T) {
		n := m2.Union(m1)
		assert.Equal(t, 3, n.Len())
		v, _ := n.Get("1")
		assert.Equal(t, 1, v)
	})
}

func BenchmarkOrderedMapInsertions(b *testing.B) {
	m := NewOrdered[int](0)
	for i := range b.N {
		k := strconv.Itoa(i)
		m.Set(k, i)
		v, _ := m.Get(k)
		assert.Equal(b, v, i)
	}
}

func BenchmarkGoMapInsertions(b *testing.B) {
	m := map[string]int{}
	for i := range b.N {
		k := strconv.Itoa(i)
		m[k] = i
		assert.Equal(b, m[k], i)
	}
}
