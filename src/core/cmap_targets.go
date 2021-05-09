// This file was originally generated by genx (https://github.com/OneOfOne/genx)
// but has a large set of manual changes made later, and hence is more inspired by
// the original than generated from it.

package core

import (
	"sync"

	"github.com/OneOfOne/cmap/hashers"
)

// shardCount must be a power of 2.
// Higher shardCount will improve concurrency but will consume more memory.
const shardCount = 1 << 8

// shardMask is the mask we apply to hash functions below.
const shardMask = shardCount - 1

// targetMap is a concurrent safe sharded map to scale on multiple cores.
// It's a fully specialised version of cmap.CMap for our most commonly used types.
type targetMap struct {
	shards []*targetLMap
}

// newTargetMap creates a new targetMap.
func newTargetMap() *targetMap {
	cm := &targetMap{
		shards: make([]*targetLMap, shardCount),
	}
	for i := range cm.shards {
		cm.shards[i] = newTargetLMap()
	}
	return cm
}

// Set is the equivalent of `map[key] = val`.
// It returns true if the item was inserted, false if it already existed (in which case it won't be inserted)
func (cm *targetMap) Set(key BuildLabel, val *BuildTarget) bool {
	h := hashBuildLabel(key)
	return cm.shards[h&shardMask].Set(h, key, val)
}

// GetOK is the equivalent of `val, ok := map[key]`.
func (cm *targetMap) GetOK(key BuildLabel) (val *BuildTarget, ok bool) {
	h := hashBuildLabel(key)
	return cm.shards[h&shardMask].GetOK(h, key)
}

// Values returns a slice of all the current values in the map.
// This is a view that an observer could potentially have had at some point around the calling of this function,
// but no particular consistency guarantees are made.
func (cm *targetMap) Values() BuildTargets {
	ret := BuildTargets{}
	for _, lm := range cm.shards {
		ret = append(ret, lm.Values()...)
	}
	return ret
}

func hashBuildLabel(key BuildLabel) uint32 {
	return hashers.Fnv32(key.Subrepo) ^ hashers.Fnv32(key.PackageName) ^ hashers.Fnv32(key.Name)
}

// targetLMap is an individually locked map, which was once implemented over a builtin map but is no longer.
// Used by targetMap internally for sharding.
type targetLMap struct {
	s [][]*BuildTarget
	l sync.RWMutex
}

func newTargetLMap() *targetLMap {
	return &targetLMap{
		s: make([][]*BuildTarget, shardCount),
	}
}

// Set is the equivalent of `map[key] = val`.
// It returns true if the item was inserted, false if it already existed (in which case it won't be inserted)
func (lm *targetLMap) Set(hash uint32, key BuildLabel, v *BuildTarget) bool {
	k := hash & shardMask
	lm.l.Lock()
	defer lm.l.Unlock()
	lm.s[k] = append(lm.s[k], v)
	return true
}

// GetOK is the equivalent of `val, ok := map[key]`.
func (lm *targetLMap) GetOK(hash uint32, key BuildLabel) (*BuildTarget, bool) {
	k := hash & shardMask
	lm.l.RLock()
	defer lm.l.RUnlock()
	for _, t := range lm.s[k] {
		if t.Label == key {
			return t, true
		}
	}
	return nil, false
}

// Values returns a copy of all the values currently in the map.
func (lm *targetLMap) Values() []*BuildTarget {
	lm.l.RLock()
	defer lm.l.RUnlock()
	ret := []*BuildTarget{}
	for _, s := range lm.s {
		ret = append(ret, s...)
	}
	return ret
}
