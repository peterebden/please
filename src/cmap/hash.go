package cmap

import (
	"unsafe"
)

const prime32 = 16777619
const initial = uint32(2166136261)

// Fnv32 returns a 32-bit FNV-1 hash of a string.
// This is a convenient hash function for a Map.
func Fnv32(s string) uint32 {
	hash := initial
	for i := 0; i < len(s); i++ {
		hash *= prime32
		hash ^= uint32(s[i])
	}
	return hash
}

// Fnv32s returns a 32-bit FNV-1 hash of the concatenation of a series of strings.
// This is a convenient hash function for a Map based on a struct containing multiple strings.
func Fnv32s(s ...string) uint32 {
	hash := initial
	for _, x := range s {
		for i := 0; i < len(x); i++ {
			hash *= prime32
			hash ^= uint32(x[i])
		}
	}
	return hash
}

// Borrow the runtime's builtin fast object hasher.
//go:linkname runtimehash runtime.memhash
//go:noescape
func runtimehash(p unsafe.Pointer, seed, s uintptr) uintptr

// Generic wrapper over the runtime version
func hash[K any](k K, seed int64) uint64 {
	return uint64(runtimehash(unsafe.Pointer(&k), uintptr(seed), uintptr(unsafe.Sizeof(k))))
}
