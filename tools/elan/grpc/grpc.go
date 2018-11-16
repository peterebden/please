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
	"tools/elan/storage"
)

// Start starts the gRPC server on the given port.
func Start(port int, storage storage.Storage, cluster Cluster) {
	s, _, lis := start(port, storage, nodeFinder)
	go s.Serve(lis)
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

// A Cluster is a minimal interface of what we require from the cluster.
type Cluster interface {
	// Nodes returns the RPC URLs of nodes currently known in the cluster.
	Nodes() []string
}

type server struct {
	cluster     Cluster
	storage     storage.Storage
	clusterInfo *pb.InfoResponse
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
