// +build !bootstrap

package fsclient

import (
	"context"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/cespare/xxhash"
	"github.com/hashicorp/go-multierror"
	"golang.org/x/sync/errgroup"
	"gopkg.in/op/go-logging.v1"

	"grpcutil"
	pb "src/remote/proto/fs"
)

var log = logging.MustGetLogger("fsclient")

// chunkSize is the chunk size we request from the server.
// According to https://github.com/grpc/grpc.github.io/issues/371 a good size might be 16-64KB.
// This is important later on for us to size buffers appropriately.
const chunkSize = 32 * 1024

// NewClient creates and returns a new client based on the given URL touchpoints that it can
// initialise from.
// N.B. Errors are not returned here since the initialisation is done asynchronously.
func NewClient(urls []string) Client {
	c := &client{urls: urls}
	go c.Init()
	return c
}

// A client is the implementation of the Client interface.
type client struct {
	nodes    []*node
	ranges   []hashRange
	initOnce sync.Once
	urls     []string
}

func (c *client) Get(filenames []string, hash []byte) ([]io.Reader, error) {
	c.Init()
	h := xxhash.Sum64(hash)
	nodes, err := c.findNodes(h)
	if err != nil {
		return nil, err
	}
	// Fast path if we get a single file
	if len(filenames) == 1 {
		r, err := c.getFile(h, nodes, filenames[0])
		return []io.Reader{r}, err
	}

	var g errgroup.Group
	ret := make([]io.Reader, len(filenames))
	for i, filename := range filenames {
		i := i
		filename := filename
		g.Go(func() error {
			r, err := c.getFile(h, nodes, filename)
			ret[i] = r
			return err
		})
	}
	return ret, g.Wait()
}

// getFile retrieves a single file from the remote cluster.
// The returned Reader is always nil if err is not nil.
func (c *client) getFile(hash uint64, nodes []*node, filename string) (io.Reader, error) {
	// Try each of the nodes in turn
	var e error
	for _, n := range nodes {
		n.Init()
		stream, err := n.Client.Get(context.Background(), &pb.GetRequest{Hash: hash, Name: filename, ChunkSize: chunkSize})
		if err != nil {
			e = multierror.Append(err, e)
			continue
		}
		return &reader{stream: stream}, nil
	}
	return nil, e
}

func (c *client) Put(filename string, hash []byte, content io.Reader) error {
	c.Init()
	h := xxhash.Sum64(hash)
	nodes, err := c.findNodes(h)
	if err != nil {
		return err
	}
	// Try each of the nodes until we find one that works.
	var e error
nodeloop:
	for _, node := range nodes {
		node.Init()
		stream, err := node.Client.Put(context.Background())
		if err != nil {
			e = multierror.Append(err, e)
			continue
		}
		buf := make([]byte, chunkSize)
		for {
			n, err := content.Read(buf)
			if n > 0 {
				if err := stream.Send(&pb.PutRequest{Hash: h, Name: filename, Chunk: buf[:n]}); err != nil {
					// This is most likely their error rather than ours, so we try another
					// replica (this makes us robust to one of them going down during transfer)
					log.Warning("Error sending file to remote storage: %s", err)
					e = multierror.Append(err, e)
					continue nodeloop
				}
			}
			if err != nil {
				if err == io.EOF {
					_, err := stream.CloseAndRecv()
					return err
				}
				// Non-EOF error meand the file is incomplete, so we must signal that
				// (there is no way in the gRPC API to break off the stream unsuccessfully
				// so the signaling is in-band).
				stream.Send(&pb.PutRequest{Cancel: true})
				return err
			}
		}
	}
	return e
}

// Init ensures the client is initialised.
func (c *client) Init() {
	c.initOnce.Do(c.init)
}

func (c *client) init() {
	var e error
	// Try the URLs one by one until we find one that works.
	for _, url := range c.urls {
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
	log.Fatalf("Failed to connect to remote FS server: %s", e)
}

// setupTopology sets up the definition of the hash ranges of the remote.
func (c *client) setupTopology(client pb.RemoteFSClient, info *pb.InfoResponse) {
	c.nodes = make([]*node, len(info.Node))
	for i, n := range info.Node {
		c.nodes[i] = &node{
			Address: n.Address,
			Name:    n.Name,
		}
		if n.Address == info.ThisNode.Address {
			c.nodes[i].Client = client
		}
		for _, r := range n.Ranges {
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

// findNodes returns all the nodes that can handle a particular hash.
func (c *client) findNodes(h uint64) ([]*node, error) {
	start := sort.Search(len(c.ranges), func(i int) bool { return c.ranges[i].Start > h })
	ret := []*node{}
	for _, r := range c.ranges[start:] {
		if h > r.End {
			break
		}
		ret = append(ret, r.Node)
	}
	if len(ret) == 0 {
		// We should not normally get here - it implies we were given an incomplete set of nodes
		// that do not cover the full hash space (or possibly so many are down that there's a hole)
		return nil, fmt.Errorf("Cannot find range for hash %x (broken ring?)", h)
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

// A reader implements an io.Reader on top of a gRPC stream.
type reader struct {
	stream pb.RemoteFS_GetClient
	buf    []byte
}

// Read implements the io.Reader interface
func (r *reader) Read(b []byte) (int, error) {
	// first, dispatch anything we have handy
	if len(r.buf) > 0 {
		if len(r.buf) > len(b) {
			// we have more than enough for this request
			copy(b, r.buf[:len(b)])
			r.buf = r.buf[len(b):]
			return len(b), nil
		}
		// Otherwise, the docs suggest that Read conventionally returns what's available,
		// which is great for us since it's simpler to do.
		n := len(r.buf)
		copy(b, r.buf)
		r.buf = r.buf[:0]
		return n, nil
	}
	// If we get here we have nothing cached and need to read some more.
	resp, err := r.stream.Recv()
	if err != nil {
		return 0, err
	} else if len(resp.Chunk) <= len(b) {
		// We haven't read more than they want, so just send it all back now (as above,
		// it's conventional to send this rather than requesting another chunk).
		copy(b, resp.Chunk)
		return len(resp.Chunk), nil
	}
	// If we get here we have read more than requested and must store some.
	copy(b, resp.Chunk[:len(b)])
	r.buf = resp.Chunk[len(b):]
	return len(b), nil
}
