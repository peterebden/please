package grpc

import (
	"sync"

	pb "github.com/thought-machine/please/src/remote/proto/fs"
)

// A Fan implements communication of messages to a number of channels.
// The zero Fan is safe for use.
type Fan struct {
	chs   []chan *pb.InfoResponse
	mutex sync.Mutex
}

// Add adds a new receiver to the fan and returns it.
// Any messages passed to Broadcast will be forwarded until Remove is called.
func (f *Fan) Add() <-chan *pb.InfoResponse {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	ch := make(chan *pb.InfoResponse, 10) // Buffering a bit makes testing a lot easier.
	f.chs = append(f.chs, ch)
	return ch
}

// Remove removes the given receiver from the fan. It will no longer receive updates.
func (f *Fan) Remove(ch <-chan *pb.InfoResponse) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	for i, c := range f.chs {
		if c == ch {
			close(c)
			f.chs[i] = f.chs[len(f.chs)-1]
			f.chs = f.chs[:len(f.chs)-1]
			return
		}
	}
	log.Warning("Missing entry in Fan.Remove")
}

// Broadcast broadcasts a message to all registered receivers.
func (f *Fan) Broadcast(msg *pb.InfoResponse) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	for _, ch := range f.chs {
		ch <- msg
	}
}
