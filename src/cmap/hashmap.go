// The code in this file is originally taken from https://github.com/tidwall/hashmap and
// used under the ISC-style licence found there.
//
// The primary modifications are to allow the hash to be passed in from outside, so we
// only calculate it once. This obviously isn't possible to implement without intrusive API
// changes, hence we need to copy this.
// Some other general simplification has gone on since we don't require all use cases of the original.

package cmap

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
func (e *entry[K, V]) setDIB(dib int) int {
	e.hdib = e.hdib>>dibBitSize<<dibBitSize | uint64(dib)&maxDIB
	return int(e.hdib)
}
func (e *entry[K, V]) setHash(hash int) {
	e.hdib = uint64(hash)<<dibBitSize | e.hdib&maxDIB
}
func makeHDIB(hash, dib int) uint64 {
	return uint64(hash)<<dibBitSize | uint64(dib)&maxDIB
}

// hashmap is a hashmap. Like map[string]interface{}
type hashmap[K comparable, V any] struct {
	cap     int
	length  int
	mask    int
	growAt  int
	buckets []entry[K, V]
}

// newHashmap returns a new hashmap. Like map[string]interface{}
func newHashmap[K comparable, V any](cap int) *hashmap[K, V] {
	sz := 8
	for sz < cap {
		sz *= 2
	}
	return &hashmap[K, V]{
		cap:     cap,
		buckets: make([]entry[K, V], sz),
		mask:    sz - 1,
		growAt:  int(float64(sz) * loadFactor),
	}
}

func (m *hashmap[K, V]) resize(newCap int) {
	nmap := newHashmap[K, V](newCap)
	for i := 0; i < len(m.buckets); i++ {
		if m.buckets[i].dib() > 0 {
			nmap.set(m.buckets[i].hash()<<dibBitSize, m.buckets[i].key, m.buckets[i].value)
		}
	}
	cap := m.cap
	*m = *nmap
	m.cap = cap
}

// Set assigns a value to a key.
// Returns the previous value, or false when no value was assigned.
func (m *hashmap[K, V]) Set(key K, value V, hash int) {
	if m.length >= m.growAt {
		m.resize(len(m.buckets) * 2)
	}
	m.set(hash, key, value)
}

func (m *hashmap[K, V]) set(hash int, key K, value V) {
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

// Get returns a pointer to a value.
// The pointer is not stable and shouldn't be used after any further calls to the map.
// The second return value is true if the value was newly inserted.
func (m *hashmap[K, V]) Get(key K, hash int) (value *V, inserted bool) {
	e := entry[K, V]{
		hdib: makeHDIB(hash>>dibBitSize, 1),
		key:  key,
	}
	hash = e.hash()
	edib := e.dib()
	i := hash & m.mask
	inserted = true // Assume this is true unless we find it below
	for {
		bdib := m.buckets[i].dib()
		// If bucket dib is zero, that means it's empty and we insert here.
		if bdib == 0 {
			m.buckets[i] = e
			if value == nil {
				m.length++
				value = &m.buckets[i].value
			}
			return
		}
		// If hash matches and key matches then we've found it
		if m.buckets[i].hash() == hash && m.buckets[i].key == key {
			if value == nil {
				value = &m.buckets[i].value
				inserted = false
			}
			return
		}
		// If the bucket's dib is less than our dib, then we're inserting here and displacing this entry.
		if bdib < edib {
			e, m.buckets[i] = m.buckets[i], e
			if value == nil {
				value = &m.buckets[i].value
			}
		}
		i = (i + 1) & m.mask
		edib = e.setDIB(e.dib() + 1)
	}
}

// Len returns the number of values in map.
func (m *hashmap[K, V]) Len() int {
	return m.length
}

// Values returns all values as a slice
func (m *hashmap[K, V]) Values() []V {
	values := make([]V, 0, m.length)
	for i := 0; i < len(m.buckets); i++ {
		if m.buckets[i].dib() > 0 {
			values = append(values, m.buckets[i].value)
		}
	}
	return values
}
