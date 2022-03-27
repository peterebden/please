// Package cmap contains a thread-safe concurrent awaitable map.
// It is optimised for large maps (e.g. tens of thousands of entries) in highly
// contended environments; for smaller maps another implementation may do better.
//
// Only slightly ad-hoc testing has shown it to outperform sync.Map for our uses
// due to less contention. It is also specifically useful in cases where a caller
// wants to be able to await items entering the map (and not having to poll it to
// find out when another goroutine may insert them).
package cmap

import (
	"fmt"
	"sync"

	"github.com/thought-machine/please/src/cmap/hashmap"
)

// DefaultShardCount is a reasonable default shard count for large maps.
const DefaultShardCount = 1 << 8

// SmallShardCount is a shard count useful for relatively small maps.
const SmallShardCount = 4

// A Map is the top-level map type. All functions on it are threadsafe.
// It should be constructed via New() rather than creating an instance directly.
type Map[K comparable, V any, H func(key K) uint32] struct {
	shards []shard[K, V]
	hasher H
	mask   uint32
}

// New creates a new Map using the given hasher to hash items in it.
// The shard count must be a power of 2; it will panic if not.
// Higher shard counts will improve concurrency but consume more memory.
// The DefaultShardCount of 256 is reasonable for a large map.
func New[K comparable, V any, H func(key K) uint32](shardCount uint32, hasher H) *Map[K, V, H] {
	mask := shardCount - 1
	if (shardCount & mask) != 0 {
		panic(fmt.Sprintf("Shard count %d is not a power of 2", shardCount))
	}
	m := &Map[K, V, H]{
		shards: make([]shard[K, V], shardCount),
		hasher: hasher,
		mask:   mask,
	}
	for i := range m.shards {
		m.shards[i].m = hashmap.New[K, awaitableValue[V]](int(shardCount))
	}
	return m
}

// Add adds the new item to the map.
// It returns true if the item was inserted, false if it already existed (in which case it won't be inserted)
func (m *Map[K, V, H]) Add(key K, val V) bool {
	h := m.hasher(key)
	return m.shards[h&m.mask].Set(key, val, false, uint64(h))
}

// Set is the equivalent of `map[key] = val`.
// It always overwrites any key that existed before.
func (m *Map[K, V, H]) Set(key K, val V) {
	h := m.hasher(key)
	m.shards[h&m.mask].Set(key, val, true, uint64(h))
}

// Get returns the value corresponding to the given key, or its zero value if
// the key doesn't exist in the map.
func (m *Map[K, V, H]) Get(key K) V {
	h := m.hasher(key)
	v, _, _ := m.shards[h&m.mask].Get(key, uint64(h))
	return v
}

// GetOrWait returns the value or, if the key isn't present, a channel that it can be waited
// on for. The caller will need to call Get again after the channel closes.
// If the channel is non-nil, then val will exist in the map; otherwise it will have its zero value.
// The third return value is true if this is the first call that is awaiting this key.
// It's always false if the key exists.
func (m *Map[K, V, H]) GetOrWait(key K) (val V, wait <-chan struct{}, first bool) {
	h := m.hasher(key)
	return m.shards[h&m.mask].Get(key, uint64(h))
}

// Values returns a slice of all the current values in the map.
// No particular consistency guarantees are made.
func (m *Map[K, V, H]) Values() []V {
	ret := []V{}
	for _, shard := range m.shards {
		ret = append(ret, shard.Values()...)
	}
	return ret
}

// An awaitableValue represents a value in the map & an awaitable channel for it to exist.
type awaitableValue[V any] struct {
	Val  V
	Wait chan struct{}
}

// A shard is one of the individual shards of a map.
type shard[K comparable, V any] struct {
	m *hashmap.Map[K, awaitableValue[V]]
	l sync.Mutex
}

// Set is the equivalent of `map[key] = val`.
// It returns true if the item was inserted, false if it was not (because an existing one was found
// and overwrite was false).
func (s *shard[K, V]) Set(key K, val V, overwrite bool, hash uint64) bool {
	s.l.Lock()
	defer s.l.Unlock()
	if existing, present := s.m.Get(key, int(hash)); present {
		if existing.Wait == nil {
			if !overwrite {
				return false // already added
			}
			existing.Val = val
			s.m.Set(key, awaitableValue[V]{Val: val}, int(hash))
			return true
		}
		// Hasn't been added, but something is waiting for it to be.
		s.m.Set(key, awaitableValue[V]{Val: val}, int(hash))
		close(existing.Wait)
		existing.Wait = nil
		return true
	}
	s.m.Set(key, awaitableValue[V]{Val: val}, int(hash))
	return true
}

// Get returns the value for a key or, if not present, a channel that it can be waited
// on for.
// Exactly one of the target or channel will be returned.
// The third value is true if it is the first call that is waiting on this value.
func (s *shard[K, V]) Get(key K, hash uint64) (val V, wait <-chan struct{}, first bool) {
	s.l.Lock()
	defer s.l.Unlock()
	if v, ok := s.m.Get(key, int(hash)); ok {
		return v.Val, v.Wait, false
	}
	ch := make(chan struct{})
	s.m.Set(key, awaitableValue[V]{Wait: ch}, int(hash))
	wait = ch
	first = true
	return
}

// Values returns a copy of all the targets currently in the map.
func (s *shard[K, V]) Values() []V {
	s.l.Lock()
	defer s.l.Unlock()
	vals := s.m.Values()
	ret := make([]V, 0, len(vals))
	for _, v := range vals {
		if v.Wait == nil {
			ret = append(ret, v.Val)
		}
	}
	return ret
}
