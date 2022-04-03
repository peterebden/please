package cmap

// A Set is a specialisation of Map that doesn't have values.
type Set[K comparable] struct {
	m *Map[K, struct{}]
}

// NewSet creates a new Set.
// The shard count must be a power of 2; it will panic if not.
// Higher shard counts will improve concurrency but consume more memory.
// The DefaultShardCount of 256 is reasonable for a large set.
func NewSet[K comparable](shardCount uint64, hasher func(K) uint64) *Set[K] {
	return &Set[K]{
		m: New[K, struct{}](shardCount, hasher),
	}
}

// Add adds the new item to the set.
// It returns true if the item was inserted, false if it already existed (in which case it won't be inserted)
func (s *Set[K]) Add(key K) bool {
	return s.m.Add(key, struct{}{})
}

// GetOrWait returns a channel that the caller can wait upon for the given key to be added.
// It returns nil if the key has already been added to the set.
// Similarly to Map, the second argument is true if this is the first call to Wait() for this key.
func (s *Set[K]) GetOrWait(key K) (<-chan struct{}, bool) {
	_, ch, first := s.m.GetOrWait(key)
	return ch, first
}

// Wait is like GetOrWait but waits for the channel to be closed (if it's not nil)
func (s *Set[K]) Wait(key K) {
	if _, ch, _ := s.m.GetOrWait(key); ch != nil {
		<-ch
	}
}

// Signal closes the channels of anyone who's previously called Wait().
// If nobody has, the key is not inserted.
func (s *Set[K]) Signal(key K) {
	s.m.shards[s.m.hasher(key)&s.m.mask].Signal(key)
}
