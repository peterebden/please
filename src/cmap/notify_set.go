package cmap

// A NotifySet is a specialisation of Map that is used to notify callers when something enters the set.
type NotifySet[K comparable] struct {
	m *Map[K, chan struct{}]
}

// NewNotifySet creates a new map which is used to signal others when an event occurs.
func NewNotifySet[K comparable](shardCount uint64, hasher func(key K) uint64) NotifySet[K] {
	return NotifySet[K]{
		m: New[K, chan struct{}](shardCount, hasher),
	}
}

// Notify notifies anyone waiting that the given key is now done.
func (m NotifySet[K]) Notify(key K) {
	if ch := m.m.Get(key); ch != nil {
		close(ch)
	}
}

// Add adds a new item to be notified about.
func (m NotifySet[K]) Add(key K) {
	m.m.Add(key, make(chan struct{}))
}

// Wait checks if a notification channel has been set for this key. If so, it waits for it and returns true.
// Otherwise, it returns false.
func (m NotifySet[K]) Wait(key K) bool {
	if ch := m.m.Get(key); ch != nil {
		<-ch
		return true
	}
	return false
}

// AddOrWait adds a new item to be notified about.
// It returns true if it was newly added and false if not.
// If it's not added then this call blocks until something else does add it.
func (m NotifySet[K]) AddOrWait(key K) bool {
	if ch, added := m.m.Add(key, make(chan struct{})); !added {
		<-ch
		return false
	}
	return true
}
