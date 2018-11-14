package fsclient

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"gopkg.in/op/go-logging.v1"

	"grpcutil"
	pb "remote/proto/fs"
)

var log = logging.MustGetLogger("fsclient")

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
	ranges   []hashRange
	initOnce sync.Once
}

func (c *client) Get(filenames []string, hash []byte) ([]io.ReadCloser, error) {
	nodes, err := c.nodes(hash)
	var g errgroup.Group
	for _, filenames := range filenames {
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
		c.setupTopology(resp)
		return
	}
	log.Fatalf("Failed to connect to remote FS server: %s", err)
}

// setupTopology sets up the definition of the hash ranges of the remote.
func (c *client) setupTopology(info *pb.InfoResponse) {
	c.ranges = make([]hashRange, len(info.Ranges))
	nodes := map[string]*node{
		info.ThisNode.Address: &node{
			Address: info.ThisNode.Address,
			Name:    info.ThisNode.Name,
			Client:  client,
		},
	}
	for i, r := range info.Ranges {
		r2 := hashRange{
			Start: r.Start,
			End:   r.End,
			Nodes: make([]*node, len(r.Nodes)),
		}
		for i, node := range r.Nodes {
			n, present := nodes[node.Address]
			if !present {
				n = &node{Address: node.Address, Name: node.Name}
				nodes[node.Address] = n
			}
			r2.Nodes[i] = n
		}
	}
}

func (c *client) nodes(hash []byte) ([]*node, error) {
	for _, r := range c.ranges {
		if h >= r.Start && h < r.End {
			return r.Nodes, nil
		}
	}
	// We should never get here - it implies we were given an incomplete set of ranges that
	// do not cover the full hash space.
	return fmt.Errorf("Cannot find range for hash %d (broken ring?)", h)
}

// A node is an internal representation of a node
type node struct {
	Address, Name string
	Client        pb.RemoteFSClient
}

// A hashRange represents a range of hashes and the node that holds them
type hashRange struct {
	Start, End uint64
	Nodes      []*node
}
