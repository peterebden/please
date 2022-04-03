// The code in this file is originally taken from https://github.com/tidwall/hashmap and
// used under the ISC-style licence found there.
//
// At this point it's been fairly heavily modified from the original to suit our use case,
// but parts of that original (and general inspiration) still exist.

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

func (e *entry[K, V]) dib() uint64 {
	return e.hdib & maxDIB
}

func (e *entry[K, V]) hash() uint64 {
	return e.hdib >> dibBitSize
}

func (e *entry[K, V]) setDIB(dib uint64) {
	e.hdib = e.hdib>>dibBitSize<<dibBitSize | uint64(dib)&maxDIB
}

func (e *entry[K, V]) setHash(hash uint64) {
	e.hdib = hash<<dibBitSize | e.hdib&maxDIB
}

func makeHDIB(hash, dib uint64) uint64 {
	return hash<<dibBitSize | dib&maxDIB
}

// hashmap is a hashmap. Like map[string]interface{}
type hashmap[K comparable, V any] struct {
	cap     int
	length  int
	mask    uint64
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
		mask:    uint64(sz - 1),
		growAt:  int(float64(sz) * loadFactor),
	}
}

func (m *hashmap[K, V]) resize(newCap int) {
	nmap := newHashmap[K, V](newCap)
	nmap.length = 0
	for i := 0; i < len(m.buckets); i++ {
		if m.buckets[i].dib() > 0 {
			nmap.length++
			v, _ := nmap.get(m.buckets[i])
			*v = m.buckets[i].value
		}
	}
	cap := m.cap
	*m = *nmap
	m.cap = cap
}

// Get returns a pointer to a value.
// The pointer is not stable and shouldn't be used after any further calls to the map.
// The second return value is true if the value was newly inserted.
func (m *hashmap[K, V]) Get(key K, hash uint64) (value *V, inserted bool) {
	if m.length >= m.growAt {
		m.resize(len(m.buckets) * 2)
	}
	v, inserted := m.get(entry[K, V]{
		hdib: makeHDIB(hash>>dibBitSize, 1),
		key:  key,
	})
	if inserted {
		m.length++
	}
	return v, inserted
}

func (m *hashmap[K, V]) get(e entry[K, V]) (value *V, inserted bool) {
	hash := e.hash()
	edib := e.dib()
	i := hash & m.mask
	inserted = true // Assume this is true unless we find it below
	for {
		bdib := m.buckets[i].dib()
		// If bucket dib is zero, that means it's empty and we insert here.
		if bdib == 0 {
			m.buckets[i] = e
			if value == nil {
				value = &m.buckets[i].value
			}
			return
		}
		// If hash matches and key matches then we've found it
		if m.buckets[i].hash() == hash && m.buckets[i].key == e.key {
			if value == nil {
				value = &m.buckets[i].value
				inserted = false
			}
			return
		}
		// If the bucket's dib is less than our dib, then we're inserting here and displacing this entry.
		if bdib < edib {
			e, m.buckets[i] = m.buckets[i], e
			hash = e.hash()
			if value == nil {
				value = &m.buckets[i].value
			}
		}
		i = (i + 1) & m.mask
		e.setDIB(e.dib() + 1)
		edib = e.dib()
	}
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
