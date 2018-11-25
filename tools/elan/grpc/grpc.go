// Package grpc implements the gRPC server for elan which handles all the bulk
// file communication.
package grpc

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/op/go-logging.v1"

	"grpcutil"
	pb "src/remote/proto/fs"
	cpb "tools/elan/proto/cluster"
	"tools/elan/storage"
)

var log = logging.MustGetLogger("grpc")

// defaultChunkSize is the default chunk size for the server.
// According to https://github.com/grpc/grpc.github.io/issues/371 it might be 16-64KB.
const defaultChunkSize = 32 * 1024

const timeout = 5 * time.Second

// Start starts the gRPC server on the given port.
// It uses the given set of URLs for initial discovery of another node to bootstrap from.
// If seed is true then it is allowed to seed a new cluster if it fails to contact any
// other nodes; otherwise failure to contact them will be fatal.
func Start(port int, urls []string, storage storage.Storage, name, addr string, replicas int) Server {
	s, srv, lis := start(port, urls, storage, name, addr, replicas)
	go s.Serve(lis)
	return srv
}

func start(port int, urls []string, storage storage.Storage, name, addr string, replicas int) (*grpc.Server, *server, net.Listener) {
	s := grpc.NewServer()
	c, err := storage.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %s", err)
	}
	if !c.Initialised {
		c.ThisNode = &pb.Node{Name: name, Address: addr}
	}
	fs := &server{
		name:     name,
		storage:  storage,
		config:   c,
		ring:     NewRing(),
		replicas: replicas,
	}
	pb.RegisterRemoteFSServer(s, fs)
	cpb.RegisterElanServer(s, fs)
	if err := fs.Init(urls, addr); err != nil {
		log.Fatalf("Failed to initialise: %s", err)
	}
	return s, fs, grpcutil.SetupServer(s, port)
}

// A Server allows some maintenance operations on the gRPC server.
type Server interface {
	// Init initialises the server & storage by connecting to the given URLs.
	Init(urls []string, addr string) error
	// GetClusterInfo returns diagnostic information about the cluster.
	GetClusterInfo() *cpb.ClusterInfoResponse
}

type server struct {
	name     string
	storage  storage.Storage
	config   *cpb.Config
	info     *pb.InfoResponse
	ring     *Ring
	replicas int
}

func (s *server) Init(urls []string, addr string) error {
	for _, url := range urls {
		client := cpb.NewElanClient(grpcutil.Dial(url))
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		resp, err := client.Register(ctx, &cpb.RegisterRequest{Node: s.config.ThisNode})
		if err != nil {
			// Not fatal, might have failed to contact server
			log.Warning("Failed to initialise off %s: %s", url, err)
		} else if !resp.Accepted {
			// This is fatal, we've been told our config is unacceptable.
			return fmt.Errorf("Request to join cluster rejected: %s", resp.Msg)
		} else {
			return s.init(resp.Nodes)
		}
	}
	log.Error("Failed to contact any initial nodes")
	if s.config.Initialised {
		log.Warning("Config already initialised, proceeding with last known settings")
		return s.init(s.config.Nodes)
	}
	log.Warning("Seeding new cluster")
	if err := s.ring.Add(s.name, addr, nil); err != nil {
		return err
	}
	if err := s.init(s.ring.Export()); err != nil {
		return err
	}
	s.config.Initialised = true
	return s.storage.SaveConfig(s.config)
}

// init sets up the server & establishes connections to the rest of the cluster.
func (s *server) init(nodes []*pb.Node) error {
	if err := s.update(nodes); err != nil {
		return err
	}
	return s.ring.Update(nodes)
}

// update updates the server with a set of nodes.
func (s *server) update(nodes []*pb.Node) error {
	s.config.Nodes = nodes
	s.config.ThisNode = s.node(s.name)
	if s.config.ThisNode == nil {
		return fmt.Errorf("this node (%s) not included in cluster info", s.name)
	}
	s.info = &pb.InfoResponse{
		Node:     s.config.Nodes,
		ThisNode: s.config.ThisNode,
	}
	return nil
}

// node returns the node with a given name, or nil if it is not known.
func (s *server) node(name string) *pb.Node {
	for _, node := range s.config.Nodes {
		if node.Name == s.name {
			return node
		}
	}
	return nil
}

func (s *server) Info(ctx context.Context, req *pb.InfoRequest) (*pb.InfoResponse, error) {
	return s.info, nil
}

func (s *server) Get(req *pb.GetRequest, stream pb.RemoteFS_GetServer) error {
	r, err := s.storage.Load(req.Hash, req.Name)
	if os.IsNotExist(err) {
		// ensure we send back the correct gRPC error so clients can identify it.
		return status.Error(codes.NotFound, "not found")
	}
	defer r.Close()

	if req.ChunkSize < 1024 { // Small chunk size would be unwise.
		req.ChunkSize = defaultChunkSize
	}
	buf := make([]byte, req.ChunkSize)
	for {
		if _, err := r.Read(buf); err != nil {
			if err == io.EOF {
				break
			}
			return err
		} else if err := stream.Send(&pb.GetResponse{Chunk: buf}); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) Put(stream pb.RemoteFS_PutServer) error {
	return s.put(stream, s.replicas > 1)
}

func (s *server) Replicate(stream cpb.Elan_ReplicateServer) error {
	return s.put(stream, false)
}

// put implements storing a single file, optionally with replication.
func (s *server) put(stream pb.RemoteFS_PutServer, replicate bool) error {
	// Read one message to get the metadata
	req, err := stream.Recv()
	if err != nil {
		return err
	}
	w, err := s.storage.Save(req.Hash, req.Name)
	if err != nil {
		return err
	}
	if replicate {
		// TODO(peterebden): Need to communicate failures to this when unsuccessful.
		w = s.replicate(w, req.Hash, req.Name)
	}
	defer w.Close()
	if _, err := w.Write(req.Chunk); err != nil {
		return err
	}
	// Now read & return the rest of the message.
	for {
		if req, err := stream.Recv(); err != nil {
			if err == io.EOF {
				break
			}
			return err
		} else if _, err := w.Write(req.Chunk); err != nil {
			return err
		}
	}
	return stream.SendAndClose(&pb.PutResponse{})
}

// replicate replicates all writes on the given writer to the appropriate nodes.
func (s *server) replicate(w io.WriteCloser, hash uint64, name string) io.WriteCloser {
	names, clients := s.ring.FindReplicas(hash, name, s.replicas-1)
	log.Info("Replicating artifact %x to nodes %s", hash, strings.Join(names, ", "))
	channels := make([]chan *pb.PutRequest, len(clients))
	for i, client := range clients {
		// Buffering so the client doesn't have to see latency if there are slow replicas,
		// but it does mean we have to suck up memory use for them.
		channels[i] = make(chan *pb.PutRequest, 1000)
		go s.forwardMessages(channels[i], clients[i], hash, name, names[i])
	}
	return &replicaWriter{w: w, channels: channels}
}

// forwardMessages forwards writes for replication from a channel to a gRPC client.
func (s *server) forwardMessages(ch <-chan *pb.PutRequest, client cpb.ElanClient, hash uint64, filename, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	stream, err := client.Replicate(ctx)
	if err != nil {
		log.Error("Error replicating to %s", name)
		s.discard(ch)
		return
	}
	for req := range ch {
		if err := stream.Send(
	}
}

func (s *server) Register(ctx context.Context, req *cpb.RegisterRequest) (*cpb.RegisterResponse, error) {
	if req.Node == nil {
		return &cpb.RegisterResponse{Msg: "bad request, missing node field"}, nil
	} else if len(req.Node.Ranges) == 0 && s.node(req.Node.Name) == nil {
		log.Notice("Register request from %s", req.Node.Name)
		// Joining server doesn't have any knowledge of the cluster.
		client := cpb.NewElanClient(grpcutil.Dial(req.Node.Address))
		if err := s.ring.Add(req.Node.Name, req.Node.Address, client); err != nil {
			return &cpb.RegisterResponse{Msg: err.Error()}, nil
		}
		s.update(s.ring.Export())
		if err := s.storage.SaveConfig(s.config); err != nil {
			return &cpb.RegisterResponse{Msg: err.Error()}, nil
		}
		log.Notice("Node %s added to cluster", req.Node.Name)
		return &cpb.RegisterResponse{Accepted: true, Nodes: s.config.Nodes}, nil
	} else if err := s.ring.Merge(req.Node.Name, req.Node.Address, req.Node.Ranges); err != nil {
		return &cpb.RegisterResponse{Msg: err.Error()}, nil
	}
	log.Notice("Re-accepted node %s into cluster", req.Node.Name)
	s.update(s.ring.Export())
	return &cpb.RegisterResponse{Accepted: true, Nodes: s.config.Nodes}, nil
}

func (s *server) ClusterInfo(ctx context.Context, req *cpb.ClusterInfoRequest) (*cpb.ClusterInfoResponse, error) {
	if err := s.ring.Verify(); err != nil {
		return &cpb.ClusterInfoResponse{
			Msg:   err.Error(),
			Nodes: s.info.Node,
		}, nil
	}
	return &cpb.ClusterInfoResponse{
		Healthy: true,
		Nodes:   s.info.Node,
	}, nil
}

func (s *server) GetClusterInfo() *cpb.ClusterInfoResponse {
	resp, _ := s.ClusterInfo(context.Background(), nil)
	resp.Segments = s.ring.Segments()
	return resp
}

// A replicaWriter forwards writes to a writer plus a set of channels to replicas.
type replicaWriter struct {
	w        io.WriteCloser
	channels []chan *pb.PutRequest
}

func (w *replicaWriter) Write(b []byte) (int, error) {
	for _, ch := range w.channels {
		ch <- &pb.PutRequest{Chunk: b}
	}
	return w.w.Write(b)
}

func (w *replicaWriter) Close() error {
	for _, ch := range w.channels {
		close(ch)
	}
	return w.w.Close()
}
