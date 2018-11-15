// +build !bootstrap

package fsclient

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/cespare/xxhash"
	"github.com/hashicorp/go-multierror"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"gopkg.in/op/go-logging.v1"

	"grpcutil"
	pb "remote/proto/fs"
)

var log = logging.MustGetLogger("fsclient")

// Size to request chunks in (in bytes).
const chunkSize = 20 * 1024

// NewClient creates and returns a new client based on the given URL touchpoints that it can
// initialise from.
// N.B. Errors are not returned here since the initialisation is done asynchronously.
func NewClient(urls []string) (Client, error) {
	c := &client{}
	go c.Init() // Begin initialisation eagerly
	return c
}

// A client is the implementation of the Client interface.
type client struct {
	nodes    []*node
	ranges   []hashRange
	initOnce sync.Once
}

func (c *client) Get(filenames []string, hash []byte) ([]io.ReadCloser, error) {
	h := xxhash.Sum64(hash)
	nodes, err := c.nodes(h)
	if err != nil {
		return nil, err
	}
	// Fast path if we get a single file
	if len(filenames) == 1 {
		r, err := c.getFile(h, nodes, filenames[0])
		return []io.ReadCloser{r}, err
	}

	var g errgroup.Group
	ret := make([]io.ReadCloser, len(filenames))
	for i, filename := range filenames {
		i := i
		filename := filename
		g.Go(func() error {
			r, err := c.getFile(nodes, filename)
			ret[i] = r
			return err
		})
	}
	if err := g.Wait(); err != nil {
		// Close anything that we got back
		for _, r := range ret {
			if r != nil {
				r.Close()
			}
		}
		return nil, err
	}
	return ret, nil
}

// getFile retrieves a single file from the remote cluster.
// The returned ReadCloser is always nil if err is not nil (i.e. you do not need to close it
// yourself in error scenarios, only on success).
func (c *client) getFile(hash uint64, nodes []*node, filename string) (io.ReadCloser, error) {
	// Try each of the nodes in turn
	var e error
	for _, n := range nodes {
		n.Init()
		stream, err := n.Client.Get(&pb.GetRequest{Hash: hash, Name: filename, ChunkSize: chunkSize})
		if err != nil {
			e = multierror.Append(err, e)
			continue
		}

	}
}

func (c *client) Put(filename string, content io.Reader, hash []byte) error {

}

// Init ensures the client is initialised.
func (c *client) Init() {
	c.initOnce.Do(c.init)
}

func (c *client) init() {
	var e error
	// Try the URLs one by one until we find one that works.
	for _, url := range urls {
		client := pb.NewRemoteFSClient(grpcutil.Dial(url))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		resp, err := client.Info(ctx, &pb.InfoRequest{})
		if err != nil {
			multierror.Append(e, err)
			continue
		}
		c.setupTopology(client, resp)
		return
	}
	log.Fatalf("Failed to connect to remote FS server: %s", err)
}

// setupTopology sets up the definition of the hash ranges of the remote.
func (c *client) setupTopology(client pb.RemoteFSClient, info *pb.InfoResponse) {
	c.nodes = make([]node, len(info.Nodes))
	for i, n := range info.Nodes {
		c.nodes[i] = &node{
			Address: n.Address,
			Name:    n.Name,
			Ranges:  make([]hashRange, len(n.Ranges)),
		}
		if n.Address == info.ThisNode.Address {
			c.nodes[i].Client = client
		}
		for i, r := range n.Ranges {
			c.ranges = append(c.ranges, hashRange{Start: r.Start, End: r.End, Node: c.nodes[i]})
		}
	}
	// Order them all by their ranges
	sort.Slice(c.ranges, func(i, j int) bool {
		if c.ranges[i].Start != c.ranges[j].Start {
			return c.ranges[i].Start < c.ranges[j].Start
		} else if c.ranges[i].End != c.ranges[j].End {
			return c.ranges[i].End < c.ranges[j].End
		}
		return false
	})
}

// nodes returns all the nodes that can handle a particular hash.
func (c *client) nodes(h uint64) ([]*node, error) {
	start := sort.Search(len(c.ranges), func(i int) bool { return c.ranges[i].Start < h })
	ret := make([]*node, 0, 4) // slightly arbitrary size assuming a few replicas & a little overlap
	for _, r := range c.ranges[start:] {
		if h > r.End {
			break
		}
		ret = append(ret, r.Node)
	}
	if len(ret) == 0 {
		// We should not normally get here - it implies we were given an incomplete set of nodes
		// that do not cover the full hash space (or possibly so many are down that there's a hole)
		return fmt.Errorf("Cannot find range for hash %x (broken ring?)", h)
	}
	// Sort the nodes into priority order.
	// Right now this is just prioritised by whether they're initialised but later we might
	// include some kind of network topology to prefer closer nodes.
	sort.Slice(ret, func(i, j int) bool {
		ihaz := ret[i].Client != nil
		jhaz := ret[j].Client != nil
		return !(ihaz == jhaz || ihaz)
	})
	return ret, nil
}

// A hashRange represents a range of hashes that a server holds
type hashRange struct {
	Start, End uint64
	Node       *node
}

// A node is an internal representation of a node
type node struct {
	Address, Name string
	Client        pb.RemoteFSClient
	initOnce      sync.Once
}

// Init ensures that the client in this node is initialised.
func (n *node) Init() {
	n.initOnce.Do(func() {
		if n.Client == nil { // might not be if it was the initial node
			n.Client = pb.NewRemoteFSClient(grpcutil.Dial(n.Address))
		}
	})
}
