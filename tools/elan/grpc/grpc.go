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
	Init() error
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

func (s *server) Init() error {
	nodes := s.cluster.Nodes()
	s.nodes = make([]node, len(nodes))
	initialised := false
	for i, node := range nodes {
		s.nodes[i].Client = rpb.NewElanClient(grpcutil.Dial(node))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		resp, err := s.nodes[i].Client.Register(ctx, &pb.RegisterRequest{
		info, err := s.nodes[i].Client.Info(ctx, &pb.InfoRequest{})
		if err != nil {
			log.Warning("Failed to contact peer %s: %s", node, err)
			continue
		}

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
