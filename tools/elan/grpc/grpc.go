// Package grpc implements the gRPC server for elan which handles all the bulk
// file communication.
package grpc

import (
	"context"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"gopkg.in/op/go-logging.v1"

	"grpcutil"
	pb "src/remote/proto/fs"
	cpb "tools/elan/proto/cluster"
	"tools/elan/storage"
)

var log = logging.MustGetLogger("grpc")

// Start starts the gRPC server on the given port.
// It uses the given set of URLs for initial discovery of another node to bootstrap from.
// If seed is true then it is allowed to seed a new cluster if it fails to contact any
// other nodes; otherwise failure to contact them will be fatal.
func Start(port int, urls []string, storage storage.Storage, name, addr string) Server {
	s, srv, lis := start(port, urls, storage, name, addr)
	go s.Serve(lis)
	return srv
}

func start(port int, urls []string, storage storage.Storage, name, addr string) (*grpc.Server, *server, net.Listener) {
	s := grpc.NewServer()
	c, err := storage.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %s", err)
	}
	if !c.Initialised {
		c.ThisNode = &pb.Node{Name: name, Address: addr}
	}
	fs := &server{
		name:    name,
		storage: storage,
		config:  c,
		ring:    NewRing(),
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
	name    string
	storage storage.Storage
	config  *cpb.Config
	info    *pb.InfoResponse
	ring    *Ring
}

func (s *server) Init(urls []string, addr string) error {
	for _, url := range urls {
		client := cpb.NewElanClient(grpcutil.Dial(url))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
	s.config.Nodes = nodes
	for _, node := range nodes {
		if node.Name == s.name {
			s.config.ThisNode = node
			break
		}
	}
	if s.config.ThisNode == nil {
		return fmt.Errorf("this node (%s) not included in cluster info", s.name)
	}
	s.info = &pb.InfoResponse{
		Node:     s.config.Nodes,
		ThisNode: s.config.ThisNode,
	}
	return s.ring.Update(nodes)
}

func (s *server) Info(ctx context.Context, req *pb.InfoRequest) (*pb.InfoResponse, error) {
	return s.info, nil
}

func (s *server) Get(req *pb.GetRequest, stream pb.RemoteFS_GetServer) error {
	return nil
}

func (s *server) Put(stream pb.RemoteFS_PutServer) error {
	return nil
}

func (s *server) Register(ctx context.Context, req *cpb.RegisterRequest) (*cpb.RegisterResponse, error) {
	if req.Node == nil {
		return &cpb.RegisterResponse{Msg: "bad request, missing node field"}, nil
	} else if len(req.Node.Ranges) == 0 {
		log.Notice("Register request from %s", req.Node.Name)
		// Joining server doesn't have any knowledge of the cluster. Easy.
		client := cpb.NewElanClient(grpcutil.Dial(req.Node.Address))
		if err := s.ring.Add(req.Node.Name, req.Node.Address, client); err != nil {
			return &cpb.RegisterResponse{Msg: err.Error()}, nil
		}
		s.config.Nodes = s.ring.Export()
		if err := s.storage.SaveConfig(s.config); err != nil {
			return &cpb.RegisterResponse{Msg: err.Error()}, nil
		}
		return &cpb.RegisterResponse{Accepted: true, Nodes: s.ring.Export()}, nil
	}
	// TODO(peterebden): implement this.
	return &cpb.RegisterResponse{Accepted: false, Msg: "rejoining not implemented"}, nil
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
