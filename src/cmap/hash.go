package cmap

import (
	"hash/maphash"

	"github.com/cespare/xxhash/v2"
)

// XXHash calculates xxHash for a string, which is a fast high-quality hash function for a Map.
func XXHash(s string) uint64 {
	return xxhash.Sum64String(s)
}

// XXHashes calculates the xxHash for a series of strings.
func XXHashes(s ...string) uint64 {
	var result uint64
	for _, x := range s {
		result ^= xxhash.Sum64String(x)
	}
	return result
}

var seed = maphash.MakeSeed()

func Hash(s string) uint64 {
	return maphash.String(seed, s)
}

func Hashes(s ...string) uint64 {
	var h maphash.Hash
	h.SetSeed(seed)
	for _, x := range s {
		h.WriteString(x)
	}
	return h.Sum64()
}
