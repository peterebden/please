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
	m := NewOrdered[int](5)
	for i := range 20 {
		m.Set(strconv.Itoa(i), i)
	}
	for i := range 20 {
		v, present := m.Get(strconv.Itoa(i))
		assert.True(t, present)
		assert.Equal(t, i, v)
	}
}
