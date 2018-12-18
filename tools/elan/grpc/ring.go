package grpc

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"

	"github.com/hashicorp/go-multierror"

	"grpcutil"
	pb "src/remote/proto/fs"
	cpb "tools/elan/proto/cluster"
)

const (
	// numTokens is the number of tokens we generate for a new entry joining the ring.
	numTokens          = 12
	tokenRange         = math.MaxUint64 / numTokens
	numAttempts        = 10
	ringMax     uint64 = math.MaxUint64
)

type clientFactory func(string) cpb.ElanClient

// A Ring is a consistently hashed ring of values that we use to manage the
// servers in a cluster.
type Ring struct {
	segments      []segment
	nodes         []*pb.Node
	clientFactory clientFactory
	// Used to guard mutating operations on the ring.
	mutex sync.Mutex
}

// NewRing creates a new ring.
func NewRing() *Ring {
	return newRing(createClient)
}

// newRing creates a new ring, and allows specifying a function to construct new clients.
func newRing(f clientFactory) *Ring {
	return &Ring{
		clientFactory: f,
	}
}

// createClient is the default client creation function.
func createClient(address string) cpb.ElanClient {
	return cpb.NewElanClient(grpcutil.Dial(address))
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
	for _, node := range nodes {
		for _, rng := range node.Ranges {
			if name, present := m2[rng.Start]; present && name != node.Name {
				return fmt.Errorf("Incompatible ranges; we record %x as being owned by %s, but now %s claims it", rng.Start, name, node.Name)
			}
			s := segment{
				Start:  rng.Start,
				End:    rng.End,
				Name:   node.Name,
				Client: m1[node.Name],
			}
			if s.Client == nil {
				s.Client = r.clientFactory(node.Address)
			}
			segs = append(segs, s)
		}
	}
	r.segments = r.sort(segs)
	r.segments = segs
	r.nodes = nodes
	return nil
}

// Adds a new entry to the ring.
func (r *Ring) Add(name, address string, client cpb.ElanClient) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.Node(name) != nil {
		return fmt.Errorf("Attempted to re-add node %s to ring", name)
	}
	// Generate a set of tokens
	for i := 0; i < numTokens; i++ {
		if err := r.genToken(uint64(i), name, client); err != nil {
			return err
		}
	}
	r.nodes = append(r.nodes, &pb.Node{
		Name:    name,
		Address: address,
		Ranges:  r.ranges(name),
		Online:  true,
	})
	return nil
}

// Merge adds the given node into the ring, either because it's already there or it is
// joining with a set of previously allocated tokens.
func (r *Ring) Merge(name, address string, ranges []*pb.Range) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	client := r.clientFactory(address)
	if node := r.Node(name); node != nil {
		// node exists, verify it has the same set of tokens
		tokens := r.ranges(name)
		if len(ranges) != 0 {
			if len(tokens) != len(ranges) {
				return fmt.Errorf("Mismatching ranges (already registered for %d, new request has %d)", len(tokens), len(ranges))
			}
			for i, tok := range tokens {
				if ranges[i].Start != tok.Start {
					return fmt.Errorf("Mismatching token: %d / %d", tok.Start, ranges[i].Start)
				}
			}
		}
		// Update its address to whatever it says it is now (it might have changed)
		if node.Address != address {
			node.Address = address
			r.UpdateNode(name, client)
		}
		node.Online = true
		return nil
	}
	// node does not exist, add it.
	// This should be relatively rare; it implies the ring is rebuilding itself.
	segs := r.segments[:]
	for _, rng := range ranges {
		if seg := r.segments[r.find(rng.Start)]; seg.Start == rng.Start {
			return fmt.Errorf("Token %d is already claimed (by %s)", rng.Start, seg.Name)
		}
		segs = append(segs, segment{Start: rng.Start, End: rng.End, Name: name, Client: client})
	}
	r.segments = r.sort(segs)
	r.nodes = append(r.nodes, &pb.Node{
		Name:    name,
		Address: address,
		Ranges:  r.ranges(name),
		Online:  true,
	})
	return nil
}

// UpdateNode sets a new client for a node and updates its liveness.
func (r *Ring) UpdateNode(name string, client cpb.ElanClient) *pb.Node {
	if node := r.Node(name); node != nil {
		node.Online = client != nil
		for i, seg := range r.segments {
			if seg.Name == name {
				r.segments[i].Client = client
			}
		}
		return node
	}
	return nil
}

// Node returns the node of given name.
func (r *Ring) Node(name string) *pb.Node {
	for _, node := range r.nodes {
		if node.Name == name {
			return node
		}
	}
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
			// avoid falling off the end of the array
			s.End = ringMax
			if len(r.segments) == 0 {
				// We've just initialised with the first segment. It starts at the beginning.
				s.Start = 0
			} else {
				r.segments[idx-1].End = token - 1
			}
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

// ranges returns the set of owned ranges for a given node.
func (r *Ring) ranges(node string) []*pb.Range {
	ret := []*pb.Range{}
	for _, seg := range r.segments {
		if seg.Name == node {
			ret = append(ret, &pb.Range{Start: seg.Start, End: seg.End})
		}
	}
	return ret
}

// sort sorts the given set of segments & matches up their start / end points.
func (r *Ring) sort(segs []segment) []segment {
	sort.Slice(segs, func(i, j int) bool { return segs[i].Start < segs[j].Start })
	for i := range segs[:len(segs)-1] {
		segs[i].End = segs[i+1].Start - 1
	}
	return segs
}

// Export exports the current state of the ring as a proto.
func (r *Ring) Export() []*pb.Node {
	return r.ExportReplicas(1)
}

// ExportReplicas exports the current state of the ring with additional ranges for replicas.
// This obviously means that there will be overlaps when replicas > 1.
func (r *Ring) ExportReplicas(replicas int) []*pb.Node {
	ret := make([]*pb.Node, 0, len(r.nodes))
	m := make(map[string]*pb.Node, len(r.nodes))
	for _, node := range r.nodes {
		n := &pb.Node{Name: node.Name, Address: node.Address, Online: node.Online}
		m[node.Name] = n
		ret = append(ret, n)
	}
	for _, s := range r.segments {
		names := []string{s.Name}
		if replicas > 1 {
			if replicas >= len(r.nodes) {
				replicas = len(r.nodes) - 1
			}
			// Doing a log(n) find here is a little suboptimal but it's a lot easier just to
			// reuse the code that exists
			names, _ = r.FindReplicas(s.Start, replicas, "")
		}
		for _, name := range names {
			n := m[name]
			n.Ranges = append(n.Ranges, &pb.Range{Start: s.Start, End: s.End})
		}
	}
	// Order nodes by their name; it is arbitrary but means this function comes out consistently.
	sort.Slice(ret, func(i, j int) bool { return ret[i].Name < ret[j].Name })
	return ret
}

// Segments returns the current set of segments as a proto.
func (r *Ring) Segments() []*cpb.Segment {
	ret := make([]*cpb.Segment, len(r.segments))
	for i, s := range r.segments {
		ret[i] = &cpb.Segment{
			Start: s.Start,
			End:   s.End,
			Name:  s.Name,
		}
	}
	return ret
}

// Nodes returns a mapping of all currently known nodes to their clients.
func (r *Ring) Nodes() map[string]cpb.ElanClient {
	m := map[string]cpb.ElanClient{}
	for _, seg := range r.segments {
		m[seg.Name] = seg.Client
	}
	return m
}

// Find returns the node that holds the given hash.
func (r *Ring) Find(hash uint64) (string, cpb.ElanClient) {
	seg := r.segments[r.find(hash)]
	return seg.Name, seg.Client
}

// FindReplicas returns the sequence of n nodes that should hold the given hash
// excluding the given one. If not enough replicas are known then it will return
// as many as possible.
func (r *Ring) FindReplicas(hash uint64, n int, current string) ([]string, []cpb.ElanClient) {
	if n >= len(r.nodes) {
		log.Warning("Insufficient replicas available (%d requested, %d nodes known (excluding this one)", n, len(r.nodes)-1)
		n = len(r.nodes) - 1
	}
	names := make([]string, 0, n)
	clients := make([]cpb.ElanClient, 0, n)
	for idx := r.find(hash); len(names) < n; idx = (idx + 1) % len(r.segments) {
		if name := r.segments[idx].Name; name != current && !contains(names, name) {
			names = append(names, name)
			clients = append(clients, r.segments[idx].Client)
		}
	}
	return names, clients
}

func contains(haystack []string, needle string) bool {
	for _, straw := range haystack {
		if straw == needle {
			return true
		}
	}
	return false
}

// find returns the index of the segment that holds the given hash.
func (r *Ring) find(hash uint64) int {
	idx := sort.Search(len(r.segments), func(i int) bool { return r.segments[i].Start >= hash })
	if idx == 0 {
		return 0
	}
	return idx - 1
}

// Verify checks the current state of the ring and returns an error if there are any issues.
func (r *Ring) Verify() error {
	var err error
	if len(r.segments) == 0 {
		return fmt.Errorf("empty ring")
	}
	last := r.segments[0]
	if last.Start != 0 {
		err = multierror.Append(err, fmt.Errorf("does not start at zero"))
	}
	for _, segment := range r.segments[1:] {
		if segment.Start < last.End+1 {
			err = multierror.Append(err, fmt.Errorf("overlap %d-%d", last.End, segment.Start))
		} else if segment.Start > last.End+1 {
			err = multierror.Append(err, fmt.Errorf("gap from %d-%d", last.End, segment.Start))
		}
		last = segment
	}
	if last.End != ringMax {
		err = multierror.Append(err, fmt.Errorf("does not finish at %d", ringMax))
	}
	return err
}

// A segment represents a segment of the circle that one node is responsible for.
// Note that as new nodes join the cluster segments can decrease in size via their
// end moving, but the start never changes.
type segment struct {
	Start, End uint64
	Name       string
	Client     cpb.ElanClient
}
