package cmap

// A Set is a specialisation of Map that doesn't have values.
type Set[K comparable, H func(K) uint64] struct {
	m Map[K, struct{}, H]
}

// NewSet creates a new Set.
// The shard count must be a power of 2; it will panic if not.
// Higher shard counts will improve concurrency but consume more memory.
// The DefaultShardCount of 256 is reasonable for a large set.
func NewSet[K comparable, H func(K) uint64](shardCount uint64, hasher H) *Set[K, H] {

}
