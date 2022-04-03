package core

import (
	"github.com/thought-machine/please/src/cmap"
)

// A notifyMap is a type we use to get notified when
type notifyMap[K comparable] struct {
	m *cmap.Map[K, chan struct{}]
}

// newNotifyMap creates a new map which is used to signal others when an event occurs.
func newNotifyMap[K comparable](shardCount uint64, hasher func(key K) uint64) notifyMap[K] {
	return notifyMap[K]{
		m: cmap.New[K, chan struct{}](shardCount, hasher),
	}
}

// Notify notifies anyone waiting that the given key is now done.
func (m notifyMap[K]) Notify(key K) {
	if ch := m.m.Get(key); ch != nil {
		close(ch)
	}
}

// Add adds a new item to be notified about.
func (m notifyMap[K]) Add(key K) {
	m.m.Add(key, make(chan struct{}))
}

// Wait checks if a notification channel has been set for this key. If so, it waits for it and returns true.
// Otherwise, it returns false.
func (m notifyMap[K]) Wait(key K) bool {
	if ch := m.m.Get(key); ch != nil {
		<-ch
		return true
	}
	return false
}

// AddOrWait adds a new item to be notified about.
// It returns true if it was newly added and false if not.
// If it's not added then this call blocks until something else does add it.
func (m notifyMap[K]) AddOrWait(key K) bool {
	if ch, added := m.m.Add(key, make(chan struct{})); !added {
		<-ch
		return false
	}
	return true
}
