package cmap

import (
	"iter"
)

// Determined somewhat empirically.
const bucketSize = 16

// An OrderedMap is an implementation of a mapping structure in Go similar to map, but which
// retains a consistent iteration order.
// At present it's always keyed by strings.
type OrderedMap[V any] struct {
	buckets     []hashBucket[V]
	bucket0     [1]hashBucket[V]
	first, last *hashEntry[V]
	length      int
}

type hashBucket[V any] struct {
	Entries [bucketSize]hashEntry[V]
}

type hashEntry[V any] struct {
	Key   string
	Value V
	Next  *hashEntry[V]
	Hash  uint64
}

// NewOrdered returns a new ordered map with at least the given initial capacity
func NewOrdered[V any](capacity int) *OrderedMap[V] {
	m := &OrderedMap[V]{}
	if capacity > bucketSize {
		m.resize((capacity / bucketSize) + 1)
	} else {
		m.buckets = m.bucket0[:]
	}
	return m
}

// Get returns the item with given key, and true if it existed
func (m *OrderedMap[V]) Get(k string) (V, bool) {
	hash := XXHash(k)
	bucket := m.bucket(hash)
	for _, entry := range bucket.Entries {
		if entry.Hash == hash && entry.Key == k {
			return entry.Value, true
		}
	}
	var v V
	return v, false
}

// Set sets the value of the given key, overwriting if it already existed.
func (m *OrderedMap[V]) Set(k string, v V) {
	hash := XXHash(k)
	bucket := m.bucket(hash)
	for i := range bucket.Entries {
		if entry := &bucket.Entries[i]; entry.Hash == hash && entry.Key == k {
			entry.Value = v
			m.length++
			return // Don't need to reorder anything on overwrite
		} else if entry.Hash == 0 && entry.Key == "" {
			// We have reached empty entries, so insert it here
			entry.Key = k
			entry.Value = v
			entry.Hash = hash
			if m.last != nil {
				m.last.Next = entry
			}
			m.last = entry
			if m.first == nil {
				m.first = entry
			}
			m.length++
			return
		}
	}
	// If we get here, it wasn't in the bucket, and we didn't have space. Disaster! We're going
	// to have to increase our size and try again.
	m.resize(len(m.buckets) * 4)
	m.Set(k, v)
}

func (m *OrderedMap[V]) resize(capacity int) {
	m.buckets = make([]hashBucket[V], capacity)
	var last *hashEntry[V]
	for entry := m.first; entry != nil; entry = entry.Next {
		dest := m.bucket(entry.Hash)
		for i := range dest.Entries {
			if e := &dest.Entries[i]; e.Hash == 0 && e.Key == "" {
				*e = *entry
				if last != nil {
					last.Next = e
				} else {
					m.first = e
				}
				last = e
				break
			}
		}
	}
	m.last = last
}

func (m *OrderedMap[V]) bucket(hash uint64) *hashBucket[V] {
	return &m.buckets[int(hash)&(len(m.buckets)-1)] // buckets are always a power of 2
}

// Items returns an iterator over (key, value) pairs of this map.
// Modifying the map during iteration is unsupported.
func (m *OrderedMap[V]) Items() iter.Seq2[string, V] {
	return func(yield func(string, V) bool) {
		for e := m.first; e != nil; e = e.Next {
			if !yield(e.Key, e.Value) {
				return
			}
		}
	}
}

// Keys returns an iterator over the keys of this map.
// Modifying the map during iteration is unsupported.
func (m *OrderedMap[V]) Keys() iter.Seq[string] {
	return func(yield func(string) bool) {
		for e := m.first; e != nil; e = e.Next {
			if !yield(e.Key) {
				return
			}
		}
	}
}

// Values returns an iterator over the values of this map.
// Modifying the map during iteration is unsupported.
func (m *OrderedMap[V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		for e := m.first; e != nil; e = e.Next {
			if !yield(e.Value) {
				return
			}
		}
	}
}

// Len returns the current number of items in this map
func (m *OrderedMap[V]) Len() int {
	return m.length
}
