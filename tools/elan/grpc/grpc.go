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
func Start(port int, storage storage.Storage, urls []string, name string) Server {
	s, srv, lis := start(port, storage, urls, name)
	go s.Serve(lis)
	return srv
}

func start(port int, storage storage.Storage, urls []string, name string) (*grpc.Server, *server, net.Listener) {
	s := grpc.NewServer()
	c, err := storage.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %s", err)
	}
	fs := &server{
		name:    name,
		storage: storage,
		config:  c,
		ring:    NewRing(),
	}
	pb.RegisterRemoteFSServer(s, fs)
	fs.Init(urls)
	return s, fs, grpcutil.SetupServer(s, port)
}

// A Server allows some maintenance operations on the gRPC server.
type Server interface {
	// Init should be called once the cluster has initialised.
	// It initialises the server & storage by connecting to the given URLs.
	Init(urls []string) error
}

type server struct {
	name    string
	storage storage.Storage
	config  *cpb.Config
	info    *pb.InfoResponse
	ring    *Ring
}

func (s *server) Init(urls []string) error {
	// Load the current config from storage in case we've been initialised before.
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
	return s.init(s.config.Nodes)
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
	if err := s.ring.Update(nodes); err != nil {
		return err
	}
	if !s.config.Initialised {
		// We're seeding a new cluster, so issue a new set of tokens.
		if err := s.ring.Add(s.config.ThisNode.Name, s.config.ThisNode.Address, nil); err != nil {
			return err
		}
		s.config.Nodes = s.ring.Export()
		s.config.Initialised = true
		if err := s.storage.SaveConfig(s.config); err != nil {
			return err
		}
	}
	return nil
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
