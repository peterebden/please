// Package grpc implements the gRPC server for elan which handles all the bulk
// file communication.
package grpc

import (
	"context"
	"net"

	"google.golang.org/grpc"
	"gopkg.in/op/go-logging.v1"

	"grpcutil"
	pb "remote/proto/fs"
	cpb "tools/elan/proto/replication"
	"tools/elan/storage"
)

// Start starts the gRPC server on the given port.
func Start(port int, storage storage.Storage, cluster Cluster) Server {
	s, srv, lis := start(port, storage, nodeFinder)
	go s.Serve(lis)
	return srv
}

func start(port int, storage storage.Storage, cluster Cluster) (*grpc.Server, *server, net.Listener) {
	s := grpc.NewServer()
	fs := &server{
		cluster: cluster,
		storage: storage,
	}
	pb.RegisterFSClientServer(s, fs)
	return s, fs, grpcutil.SetupServer(s, port)
}

// A Server allows some maintenance operations on the gRPC server.
type Server interface {
	// Init should be called once the cluster has initialised.
	// It initialises the server & storage.
	Init(name string) error
}

// A Cluster is a minimal interface of what we require from the cluster.
type Cluster interface {
	// Nodes returns the RPC URLs of nodes currently known in the cluster.
	Nodes() []string
}

type server struct {
	cluster     Cluster
	storage     storage.Storage
	clusterInfo *pb.InfoResponse
	nodes       []node
}

func (s *server) Init(name string) error {
	nodes := s.cluster.Nodes()
	s.nodes = make([]node, 0, len(nodes))
	initialised := false
	for nodeName, url := range nodes {
		if nodeName == name {
			// don't open a gRPC channel to ourselves
			s.nodes = append(s.nodes, node{Name: nodeName})
		} else {
			s.nodes = append(s.nodes, node{
				Name:   nodeName,
				Client: rpb.NewElanClient(grpcutil.Dial(url)),
			})
		}
	}
	// We try to avoid consensus problems via this shitty method of treating the
	// lowest-named node as the master. Obviously that is pretty dodgy but it's
	// enough for now - later we'll add Raft or smthn to do it better.
	sort.Slice(s.nodes, func(i, j int) bool { return i.Name < j.Name })
	// Handle the case where we are the master.
	idx := 0
	if s.nodes[idx].Name == name {
		if len(nodes) == 1 {
		}
		// Initialise off the second node.
		// TODO(peterebden): this indicates we can't tolerate both going down.
		idx = 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := s.nodes[0].Client.Register(ctx, &pb.RegisterRequest{
		Name:   name,
		Tokens: storage.Tokens(),
	})
	if err != nil {
		log.Error("Failed to contact peer %s: %s", s.nodes[0].Name, err)
		return err
	}
}

func (s *server) Info(ctx context.Context, req *pb.InfoRequest) (*pb.InfoResponse, error) {
	return s.clusterInfo, nil
}

func (s *server) Get(req *GetRequest, stream pb.RemoteFS_GetServer) error {
	return nil
}

func (s *server) Put(stream pb.RemoteFS_PutServer) error {
	return nil
}
