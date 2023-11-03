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

	"github.com/prometheus/client_golang/prometheus"
)

// DefaultShardCount is a reasonable default shard count for large maps.
const DefaultShardCount = 1 << 8

// SmallShardCount is a shard count useful for relatively small maps.
const SmallShardCount = 4

// A Map is the top-level map type. All functions on it are threadsafe.
// It should be constructed via New() rather than creating an instance directly.
type Map[K comparable, V any] struct {
	shards []shard[K, V]
	mask   uint64
	hasher func(K) uint64
}

// New creates a new Map using the given hasher to hash items in it.
// The shard count must be a power of 2; it will panic if not.
// Higher shard counts will improve concurrency but consume more memory.
// The DefaultShardCount of 256 is reasonable for a large map.
func New[K comparable, V any](shardCount uint64, hasher func(K) uint64) *Map[K, V] {
	mask := shardCount - 1
	if (shardCount & mask) != 0 {
		panic(fmt.Sprintf("Shard count %d is not a power of 2", shardCount))
	}
	return &Map[K, V]{
		shards: make([]shard[K, V], shardCount),
		mask:   mask,
		hasher: hasher,
	}
}

// Add adds the new item to the map.
// It returns true if the item was inserted, false if it already existed (in which case it won't be inserted)
func (m *Map[K, V]) Add(key K, val V) bool {
	_, inserted := m.shards[m.hasher(key)&m.mask].Set(key, val, false)
	return inserted
}

// AddOrGet either adds a new item (if the key doesn't exist) or gets the existing one.
// It returns true if the item was inserted, false if it already existed (in which case it won't be inserted)
func (m *Map[K, V]) AddOrGet(key K, val V) (V, bool) {
	return m.shards[m.hasher(key)&m.mask].Set(key, val, false)
}

// Set is the equivalent of `map[key] = val`.
// It always overwrites any key that existed before.
func (m *Map[K, V]) Set(key K, val V) {
	m.shards[m.hasher(key)&m.mask].Set(key, val, true)
}

// Get returns the value https://github.com/peterebden/please/pull/new/generic-cmap-4corresponding to the given key, or its zero value if
// the key doesn't exist in the map.
func (m *Map[K, V]) Get(key K) V {
	v, _, _ := m.shards[m.hasher(key)&m.mask].Get(key)
	return v
}

// GetOrWait returns the value or, if the key isn't present, a channel that it can be waited
// on for. The caller will need to call Get again after the channel closes.
// If the channel is non-nil, then val will exist in the map; otherwise it will have its zero value.
// The third return value is true if this is the first call that is awaiting this key.
// It's always false if the key exists.
func (m *Map[K, V]) GetOrWait(key K) (val V, wait <-chan struct{}, first bool) {
	return m.shards[m.hasher(key)&m.mask].Get(key)
}

// Values returns a slice of all the current values in the map.
// No particular consistency guarantees are made.
func (m *Map[K, V]) Values() []V {
	ret := []V{}
	for i := 0; i < len(m.shards); i++ {
		ret = append(ret, m.shards[i].Values()...)
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
	m sync.Map // implicit type map[K]awaitableValue[V]
}

// Set is the equivalent of `map[key] = val`.
// It returns the previous value and true if the item was inserted, false if it was not
// (because an existing one was found and overwrite was false).
func (s *shard[K, V]) Set(key K, val V, overwrite bool) (V, bool) {
	if overwrite {
		if existing, present := s.m.Swap(key, awaitableValue[V]{Val: val}); present {
			texisting := existing.(awaitableValue[V])
			if texisting.Wait != nil {
				close(texisting.Wait)
			}
			return texisting.Val, true
		}
		var empty V
		return empty, true
	}
	if existing, loaded := s.m.LoadOrStore(key, awaitableValue[V]{Val: val}); loaded {
		texisting := existing.(awaitableValue[V])
		if texisting.Wait != nil {
			close(texisting.Wait)
		}
		return texisting.Val, true
	}
	var old V
	return old, true
}

// Get returns the value for a key or, if not present, a channel that it can be waited
// on for.
// Exactly one of the target or channel will be returned.
// The third value is true if it is the first call that is waiting on this value.
func (s *shard[K, V]) Get(key K) (val V, wait <-chan struct{}, first bool) {
	if v, ok := s.m.Load(key); ok {
		tv := v.(awaitableValue[V])
		getFast.Inc()
		return tv.Val, tv.Wait, false
	}
	ch := make(chan struct{})
	if v, loaded := s.m.LoadOrStore(key, awaitableValue[V]{Wait: ch}); loaded {
		tv := v.(awaitableValue[V])
		getMedium.Inc()
		return tv.Val, tv.Wait, false
	}
	wait = ch
	first = true
	return
}

// Values returns a copy of all the targets currently in the map.
func (s *shard[K, V]) Values() []V {
	ret := []V{}
	s.m.Range(func(k, v any) bool {
		if tv := v.(awaitableValue[V]); tv.Wait == nil {
			ret = append(ret, tv.Val)
		}
		return true
	})
	return ret
}

var getFast = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: "plz",
	Subsystem: "cmap",
	Name:      "get_fast",
	Help:      "Time to get an existing item from the map",
})

var getMedium = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: "plz",
	Subsystem: "cmap",
	Name:      "get_medium",
	Help:      "Time to get an existing item from the map after upgrading the lock",
})

var getSlow = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: "plz",
	Subsystem: "cmap",
	Name:      "get_slow",
	Help:      "Time to get a not existing item from the map",
})

func init() {
	prometheus.MustRegister(getFast)
	prometheus.MustRegister(getSlow)
	prometheus.MustRegister(getMedium)
}
