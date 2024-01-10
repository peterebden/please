// Package ordmap provides a generic map, similar to the builtin map but which retains an order.
package ordmap

import (
	"maps"
	"slices"
)

// A Map is a generic map which supports ordering for iteration.
// It is not safe for concurrent use.
// The zero value is safe for use.
type Map[K comparable, V any] struct {
	keys map[K]int
	objs []entry[K, V]
}

type entry[K, V any] struct {
	Key K
	Val V
}

// New returns a new Map with the given capacity preallocated.
// If capacity is unknown, the zero value for Map can be used directly.
func New[K comparable, V any](size int) *Map[K, V] {
	return &Map[K, V]{
		keys: make(map[K]int, size),
		objs: make([]entry[K, V], 0, size),
	}
}

// Len returns the number of keys currently in the map.
func (m *Map[K, V]) Len() int {
	return len(m.objs)
}

// Contains returns true if the map contains an item with key K.
func (m *Map[K, V]) Contains(key K) bool {
	_, present := m.keys[key]
	return present
}

// Get returns the item with key K.
func (m *Map[K, V]) Get(key K) (V, bool) {
	idx, present := m.keys[key]
	if !present {
		var v V
		return v, false
	}
	return m.objs[idx].Val, true
}

// Put stores a key/value pair, overwriting any existing item.
func (m *Map[K, V]) Put(key K, val V) {
	e := entry[K, V]{Key: key, Val: val}
	if idx, present := m.keys[key]; present {
		m.objs[idx] = e
		return
	}
	if m.keys == nil {
		m.keys = map[K]int{key: len(m.objs)}
	} else {
		m.keys[key] = len(m.objs)
	}
	m.objs = append(m.objs, e)
}

// Delete deletes the item with the given key from the map
// N.B. This is not efficiently implemented and runs in linear time.
func (m *Map[K, V]) Delete(key K) {
	if idx, present := m.keys[key]; present {
		delete(m.keys, key)
		m.objs = slices.Delete(m.objs, idx, idx+1)
		for k, v := range m.keys {
			if v > idx {
				m.keys[k] = v - 1
			}
		}
	}
}

// Union returns a copy of this map combined with the given one.
// All keys in that map come after keys in this map (except where there are duplicates, then keys in that map overwrite).
func (m *Map[K, V]) Union(that *Map[K, V]) *Map[K, V] {
	keys := make(map[K]int, len(m.keys)+len(that.keys))
	objs := make([]entry[K, V], len(m.objs), len(m.objs)+len(that.objs))
	for k, i := range m.keys {
		keys[k] = i
	}
	for i, o := range m.objs {
		objs[i] = o
	}
	for k, i := range that.keys {
		if idx, present := keys[k]; present {
			objs[idx] = that.objs[i]
			continue
		}
		keys[k] = len(objs)
		objs = append(objs, that.objs[i])
	}
	return &Map[K, V]{keys: keys, objs: objs}
}

// Copy creates a shallow copy of this map.
func (m *Map[K, V]) Copy() *Map[K, V] {
	return &Map[K, V]{
		objs: m.objs[:],
		keys: maps.Clone(m.keys),
	}
}

// Iter returns an iterator on this map.
// These are typically used like:
//
//	for it := m.Iter(); !it.Done(); it.Next() {
//	    key := it.Key()
//	    val := it.Val()
//	    key, val = it.Item()
//	}
//
// Behaviour is undefined if the map is modified during iteration.
func (m *Map[K, V]) Iter() Iter[K, V] {
	return Iter[K, V]{m: m}
}

// An Iter is an iterator for a map.
type Iter[K comparable, V any] struct {
	m *Map[K, V]
	i int
}

// Done returns true if this iterator has reached the end of the map.
func (it Iter[K, V]) Done() bool {
	return it.i == len(it.m.objs)
}

// Next increments this iterator and returns the next item.
func (it *Iter[K, V]) Next() {
	it.i++
}

// Key returns the key at the iterator's current position.
// It will panic if the iterator has reached its end.
func (it Iter[K, V]) Key() K {
	return it.m.objs[it.i].Key
}

// Val returns the value at the iterator's current position.
// It will panic if the iterator has reached its end.
func (it Iter[K, V]) Val() V {
	return it.m.objs[it.i].Val
}

// Item returns the key and value at the iterator's current position.
// It will panic if the iterator has reached its end.
func (it Iter[K, V]) Item() (K, V) {
	e := it.m.objs[it.i]
	return e.Key, e.Val
}
