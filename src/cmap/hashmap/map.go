// Package hashmap provides a hash table used for efficient internal implementation of cmap.
//
// The code is originally taken from https://github.com/tidwall/hashmap and used under the
// ISC-style licence found there.
//
// The primary modifications are to allow the hash to be passed in from outside, so we
// only calculate it once. This obviously isn't possible to implement without intrusive API
// changes, hence we need to copy this.
// Some other general simplification has gone on since we don't require all use cases of the original.
package hashmap

const (
	loadFactor  = 0.85                      // must be above 50%
	dibBitSize  = 16                        // 0xFFFF
	hashBitSize = 64 - dibBitSize           // 0xFFFFFFFFFFFF
	maxHash     = ^uint64(0) >> dibBitSize  // max 28,147,497,671,0655
	maxDIB      = ^uint64(0) >> hashBitSize // max 65,535
)

type entry[K comparable, V any] struct {
	hdib  uint64 // bitfield { hash:48 dib:16 }
	value V      // user value
	key   K      // user key
}

func (e *entry[K, V]) dib() int {
	return int(e.hdib & maxDIB)
}
func (e *entry[K, V]) hash() int {
	return int(e.hdib >> dibBitSize)
}
func (e *entry[K, V]) setDIB(dib int) {
	e.hdib = e.hdib>>dibBitSize<<dibBitSize | uint64(dib)&maxDIB
}
func (e *entry[K, V]) setHash(hash int) {
	e.hdib = uint64(hash)<<dibBitSize | e.hdib&maxDIB
}
func makeHDIB(hash, dib int) uint64 {
	return uint64(hash)<<dibBitSize | uint64(dib)&maxDIB
}

// Map is a hashmap. Like map[string]interface{}
type Map[K comparable, V any] struct {
	cap      int
	length   int
	mask     int
	growAt   int
	shrinkAt int
	buckets  []entry[K, V]
}

// New returns a new Map. Like map[string]interface{}
func New[K comparable, V any](cap int) *Map[K, V] {
	m := new(Map[K, V])
	m.cap = cap
	sz := 8
	for sz < m.cap {
		sz *= 2
	}
	m.buckets = make([]entry[K, V], sz)
	m.mask = len(m.buckets) - 1
	m.growAt = int(float64(len(m.buckets)) * loadFactor)
	m.shrinkAt = int(float64(len(m.buckets)) * (1 - loadFactor))
	return m
}

func (m *Map[K, V]) resize(newCap int) {
	nmap := New[K, V](newCap)
	for i := 0; i < len(m.buckets); i++ {
		if m.buckets[i].dib() > 0 {
			nmap.set(m.buckets[i].hash(), m.buckets[i].key, m.buckets[i].value)
		}
	}
	cap := m.cap
	*m = *nmap
	m.cap = cap
}

// Set assigns a value to a key.
// Returns the previous value, or false when no value was assigned.
func (m *Map[K, V]) Set(key K, value V, hash int) {
	if m.length >= m.growAt {
		m.resize(len(m.buckets) * 2)
	}
	m.set(hash, key, value)
}

func (m *Map[K, V]) set(hash int, key K, value V) {
	e := entry[K, V]{makeHDIB(hash>>dibBitSize, 1), value, key}
	hash = e.hash()
	i := hash & m.mask
	for {
		if m.buckets[i].dib() == 0 {
			m.buckets[i] = e
			m.length++
			return
		}
		if hash == m.buckets[i].hash() && e.key == m.buckets[i].key {
			m.buckets[i].value = e.value
			return
		}
		if m.buckets[i].dib() < e.dib() {
			e, m.buckets[i] = m.buckets[i], e
		}
		i = (i + 1) & m.mask
		e.setDIB(e.dib() + 1)
	}
}

// Get returns a value for a key.
// Returns false when no value has been assign for key.
func (m *Map[K, V]) Get(key K, hash int) (value V, ok bool) {
	e := entry[K, V]{makeHDIB(hash>>dibBitSize, 1), value, key}
	hash = e.hash()
	i := hash & m.mask
	for {
		if m.buckets[i].dib() == 0 {
			return value, false
		}
		if m.buckets[i].hash() == hash && m.buckets[i].key == key {
			return m.buckets[i].value, true
		}
		i = (i + 1) & m.mask
	}
}

// Len returns the number of values in map.
func (m *Map[K, V]) Len() int {
	return m.length
}

// Values returns all values as a slice
func (m *Map[K, V]) Values() []V {
	values := make([]V, 0, m.length)
	for i := 0; i < len(m.buckets); i++ {
		if m.buckets[i].dib() > 0 {
			values = append(values, m.buckets[i].value)
		}
	}
	return values
}
