// +build !bootstrap

package fsclient

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"sync"

	"github.com/cespare/xxhash"
	"github.com/hashicorp/go-multierror"
	"golang.org/x/sync/errgroup"
	"gopkg.in/op/go-logging.v1"

	"github.com/thought-machine/please/src/grpcutil"
	pb "github.com/thought-machine/please/src/remote/proto/fs"
)

var log = logging.MustGetLogger("fsclient")

// chunkSize is the chunk size we request from the server.
// According to https://github.com/grpc/grpc.github.io/issues/371 a good size might be 16-64KB.
// This is important later on for us to size buffers appropriately.
const chunkSize = 32 * 1024

// theClient is a global used for Get.
var theClient Client

// New creates and returns a new client based on the given URL touchpoints that it can
// initialise from.
// N.B. Errors are not returned here since the initialisation is done asynchronously.
func New(urls []string) Client {
	c := &client{urls: urls}
	go c.Init()
	return c
}

// Get is like New but reuses a client if one has already been created.
func Get(urls []string) Client {
	if theClient == nil {
		theClient = New(urls)
	}
	return theClient
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
			e = multierror.Append(e, err)
			continue
		}
		return &reader{stream: stream}, nil
	}
	return nil, e
}

func (c *client) GetInto(filenames []string, hash []byte, dir string) error {
	var g errgroup.Group
	rs, err := c.Get(filenames, hash)
	if err != nil {
		return err
	}
	for i, filename := range filenames {
		r := rs[i]
		filename := filename
		g.Go(func() error {
			f, err := os.Create(path.Join(dir, filename))
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(f, r)
			return err
		})
	}
	return g.Wait()
}

func (c *client) Put(filenames []string, hash []byte, contents []io.ReadSeeker) error {
	c.Init()
	h := xxhash.Sum64(hash)
	nodes, err := c.findNodes(h)
	if err != nil {
		return err
	}
	// Fast path if we get a single file
	if len(filenames) == 1 {
		return c.putFile(nodes, filenames[0], h, contents[0])
	}
	var g errgroup.Group
	for i, filename := range filenames {
		i := i
		filename := filename
		g.Go(func() error {
			return c.putFile(nodes, filename, h, contents[i])
		})
	}
	return g.Wait()
}

func (c *client) PutRelative(filenames []string, hash []byte, dir string) error {
	files := make([]io.ReadSeeker, len(filenames))
	for i, fn := range filenames {
		f, err := os.Open(path.Join(dir, fn))
		if err != nil {
			return err
		}
		defer f.Close()
		files[i] = f
	}
	return c.Put(filenames, hash, files)
}

func (c *client) putFile(nodes []*node, filename string, hash uint64, contents io.ReadSeeker) error {
	// Try each of the nodes until we find one that works.
	var e error
	for _, node := range nodes {
		node.Init()
		if stream, err := node.Client.Put(context.Background()); err != nil {
			e = multierror.Append(e, err)
		} else if err := c.writeFile(stream, filename, hash, contents); err != nil {
			// Already exists is fine (this often happens legitimately)
			if grpcutil.IsAlreadyExists(err) {
				return nil
			}
			e = multierror.Append(e, err)
			contents.Seek(0, io.SeekStart) // reset the reader
		} else {
			return nil
		}
	}
	return e
}

func (c *client) writeFile(stream pb.RemoteFS_PutClient, filename string, hash uint64, content io.ReadSeeker) error {
	buf := make([]byte, chunkSize)
	for {
		n, err := content.Read(buf)
		if n > 0 {
			if err := stream.Send(&pb.PutRequest{Hash: hash, Name: filename, Chunk: buf[:n]}); err != nil {
				// This is most likely their error rather than ours, so we try another
				// replica (this makes us robust to one of them going down during transfer)
				log.Warning("Error sending file to remote storage: %s", err)
				return err
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

// Init ensures the client is initialised.
func (c *client) Init() {
	c.initOnce.Do(c.init)
}

func (c *client) init() {
	var e error
	// Try the URLs one by one until we find one that works.
	for _, url := range c.urls {
		client := pb.NewRemoteFSClient(grpcutil.Dial(url))
		if err := c.initFrom(client); err != nil {
			multierror.Append(e, err)
		} else {
			return
		}
	}
	log.Fatalf("Failed to connect to remote FS server: %s", e)
}

// initFrom initialises using the given RPC client.
func (c *client) initFrom(client pb.RemoteFSClient) error {
	stream, err := client.Info(context.Background(), &pb.InfoRequest{})
	if err != nil {
		return err
	}
	resp, err := stream.Recv()
	if err != nil {
		return err
	}
	c.nodes, c.ranges = c.setupTopology(client, resp)
	go c.runUpdates(client, stream)
	return nil
}

// setupTopology sets up the definition of the hash ranges of the remote.
func (c *client) setupTopology(client pb.RemoteFSClient, info *pb.InfoResponse) ([]*node, []hashRange) {
	nodes := make([]*node, len(info.Node))
	ranges := []hashRange{}
	clients := make(map[string]pb.RemoteFSClient, len(c.nodes))
	// Splice in any existing clients
	for _, node := range c.nodes {
		clients[node.Name] = node.Client
	}
	for i, n := range info.Node {
		nodes[i] = &node{
			Address: n.Address,
			Name:    n.Name,
		}
		if n.Address == info.ThisNode.Address {
			nodes[i].Client = client
		} else {
			nodes[i].Client = clients[n.Name]
		}
		for _, r := range n.Ranges {
			ranges = append(ranges, hashRange{Start: r.Start, End: r.End, Node: nodes[i]})
		}
	}
	// Order them all by their ranges
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].Start != ranges[j].Start {
			return ranges[i].Start < ranges[j].Start
		} else if ranges[i].End != ranges[j].End {
			return ranges[i].End < ranges[j].End
		}
		return false
	})
	return nodes, ranges
}

// runUpdates continually reads the given stream for updates to the ring topology.
func (c *client) runUpdates(client pb.RemoteFSClient, stream pb.RemoteFS_InfoClient) {
	for {
		if msg, err := stream.Recv(); err != nil {
			// Be a bit more gentle if they disconnected us gracefully.
			if err == io.EOF {
				log.Warning("Remote FS server disconnected, attempting reconnect...")
			} else {
				log.Error("Lost communication with remote FS server: %s", err)
			}
			c.findNewClient(client)
			break
		} else {
			log.Debug("Got state update from remotefs")
			c.nodes, c.ranges = c.setupTopology(client, msg)
		}
	}
}

// findNewClient locates a new client to communicate with the cluster on.
// The given one is excluded from the search.
// If successful then updates are streamed in using it.
func (c *client) findNewClient(client pb.RemoteFSClient) {
	found := false
	for _, node := range c.nodes {
		if node.Client != nil {
			if node.Client == client {
				node.Client = nil
			} else if !found {
				if err := c.initFrom(node.Client); err != nil {
					log.Warning("Failed to reconnect to remote FS via %s: %s", node.Address, err)
				} else {
					found = true
				}
			}
		}
	}
	// At this point if found is still false we could try to loop again and connect one,
	// but it's starting to become a bit of a mess in terms of which ones would be worth
	// trying, and that mostly only solves the case where we get disconnected very early
	// on (before we've connected many clients). We will settle for some messages.
	if found {
		log.Notice("Remote FS server reconnected")
	} else {
		// It's probably all stuffed really if we are here, but there isn't much to be
		// done from here. Most likely something bad has happened to the network or the
		// cluster and the user is going to find out pretty soon anyway.
		log.Error("Cannot reconnect any remote FS server")
	}
}

// findNodes returns all the nodes that can handle a particular hash.
func (c *client) findNodes(h uint64) ([]*node, error) {
	start := sort.Search(len(c.ranges), func(i int) bool { return c.ranges[i].Start >= h }) - 1
	if start == -1 {
		start = 0
	}
	ret := []*node{}
	for _, r := range c.ranges[start:] {
		if h < r.Start {
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

// IsNotFound returns true if the given error represents a failure to find an artifact
// (which may be of interest to callers since it may be expected in some cases).
func IsNotFound(err error) bool {
	if me, ok := err.(*multierror.Error); ok {
		for _, e := range me.Errors {
			if !grpcutil.IsNotFound(e) {
				return false
			}
		}
		return true
	}
	return grpcutil.IsNotFound(err)
}
