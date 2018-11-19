package grpc

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"

	"grpcutil"
	pb "src/remote/proto/fs"
	cpb "tools/elan/proto/cluster"
)

const (
	// numTokens is the number of tokens we generate for a new entry joining the ring.
	numTokens   = 12
	tokenRange  = math.MaxUint64 / numTokens
	numAttempts = 10
)

// A Ring is a consistently hashed ring of values that we use to manage the
// servers in a cluster.
type Ring struct {
	segments  []segment
	addresses map[string]string
	// Used to guard mutating operations on the ring.
	mutex sync.Mutex
}

// NewRing creates a new ring.
func NewRing() *Ring {
	return &Ring{addresses: map[string]string{}}
}

// Updates this ring to match the given proto description.
// It returns an error if the input is incompatible with its current state.
func (r *Ring) Update(nodes []*pb.Node) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	m1 := map[string]cpb.ElanClient{}
	m2 := map[uint64]string{}
	for _, seg := range r.segments {
		m1[seg.Name] = seg.Client
		m2[seg.Start] = seg.Name
	}
	segs := []segment{}
	addrs := map[string]string{}
	for _, node := range nodes {
		for _, r := range node.Ranges {
			if name, present := m2[r.Start]; present && name != node.Name {
				return fmt.Errorf("Incompatible ranges; we record %x as being owned by %s, but now %s claims it", r.Start, name, node.Name)
			}
			s := segment{
				Start:  r.Start,
				End:    r.End,
				Name:   node.Name,
				Client: m1[node.Name],
			}
			if s.Client == nil {
				s.Client = cpb.NewElanClient(grpcutil.Dial(node.Address))
			}
			segs = append(segs, s)
			addrs[node.Name] = node.Address
		}
	}
	r.segments = segs
	r.addresses = addrs
	return nil
}

// Adds a new entry to the ring.
func (r *Ring) Add(name, address string, client cpb.ElanClient) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if _, present := r.addresses[name]; present {
		return fmt.Errorf("Attempted to re-add node %s to ring", name)
	}
	// Generate a set of tokens
	for i := 0; i < numTokens; i++ {
		if err := r.genToken(uint64(i), name, client); err != nil {
			return err
		}
	}
	r.addresses[name] = address
	return nil
}

func (r *Ring) genToken(tokenIndex uint64, name string, client cpb.ElanClient) error {
	s := segment{
		Name:   name,
		Client: client,
	}
	for i := 0; i < numAttempts; i++ {
		token := uint64(rand.Int63n(tokenRange)) + tokenIndex*tokenRange
		s.Start = token
		idx := sort.Search(len(r.segments), func(i int) bool { return r.segments[i].Start >= token })
		if idx >= len(r.segments) {
			if len(r.segments) == 0 {
				// We've just initialised with the first segment. It starts at the beginning.
				s.Start = 0
			}
			// avoid falling off the end of the array
			s.End = math.MaxUint64
			r.segments = append(r.segments, s)
			return nil
		} else if r.segments[idx].Start == token {
			continue // Can't issue a token that is already issued to someone else
		} else {
			s.End = r.segments[idx].Start - 1
			r.segments = append(r.segments, s)
			copy(r.segments[idx+1:], r.segments[idx:])
			r.segments[idx] = s
			r.segments[idx-1].End = token - 1
			return nil
		}
	}
	return fmt.Errorf("Couldn't generate a new token after %d tries", numAttempts)
}

// Export exports the current state of the ring as a proto.
func (r *Ring) Export() []*pb.Node {
	ret := make([]*pb.Node, 0, len(r.addresses))
	m := make(map[string]*pb.Node, len(r.addresses))
	for name, address := range r.addresses {
		n := &pb.Node{Name: name, Address: address}
		m[name] = n
		ret = append(ret, n)
	}
	for _, s := range r.segments {
		n := m[s.Name]
		n.Ranges = append(n.Ranges, &pb.Range{Start: s.Start, End: s.End})
	}
	return ret
}

// Find returns the node that holds the given hash.
func (r *Ring) Find(hash uint64) (string, cpb.ElanClient) {
	seg := r.segments[r.find(hash)]
	return seg.Name, seg.Client
}

// FindN returns the sequence of n nodes that hold the given hash,
// i.e. the one that holds it and the n-1 immediately following.
func (r *Ring) FindN(hash uint64, n int) []cpb.ElanClient {
	ret := make([]cpb.ElanClient, 0, n)
	idx := r.find(hash)
	for i := 0; i < n; i++ {
		ret = append(ret, r.segments[idx].Client)
		idx = (idx + 1) % len(r.segments)
	}
	return ret
}

// find returns the index of the segment that holds the given hash.
func (r *Ring) find(hash uint64) int {
	return sort.Search(len(r.segments), func(i int) bool { return r.segments[i].Start >= hash })
}

// A segment represents a segment of the circle that one node is responsible for.
// Note that as new nodes join the cluster segments can decrease in size via their
// end moving, but the start never changes.
type segment struct {
	Start, End uint64
	Name       string
	Client     cpb.ElanClient
}
