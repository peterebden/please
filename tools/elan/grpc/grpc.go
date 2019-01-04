// Package grpc implements the gRPC server for elan which handles all the bulk
// file communication.
package grpc

import (
	"context"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/op/go-logging.v1"

	"github.com/thought-machine/please/src/grpcutil"
	pb "github.com/thought-machine/please/src/remote/proto/fs"
	"github.com/thought-machine/please/tools/elan/cluster"
	cpb "github.com/thought-machine/please/tools/elan/proto/cluster"
	"github.com/thought-machine/please/tools/elan/storage"
)

var log = logging.MustGetLogger("grpc")

// defaultChunkSize is the default chunk size for the server.
// According to https://github.com/grpc/grpc.github.io/issues/371 a good size might be 16-64KB.
const defaultChunkSize = 32 * 1024

// bufferSize is the number of messages we buffer to send to the replicas.
// Increasing this reduces the replication lag clients see, but also increases our memory requirements.
const bufferSize = 1000

const timeout = 5 * time.Second

// Start starts the gRPC server on the given port.
func Start(port int, ring *cluster.Ring, config *cpb.Config, storage storage.Storage, replicas int) Server {
	s, srv, lis := start(port, ring, config, storage, replicas)
	go s.Serve(lis)
	return srv
}

func start(port int, ring *cluster.Ring, config *cpb.Config, storage storage.Storage, replicas int) (*grpc.Server, *server, net.Listener) {
	s := grpc.NewServer()
	lis := grpcutil.SetupServer(s, port)
	fs := &server{
		name:     config.ThisNode.Name,
		storage:  storage,
		replicas: replicas,
		ring:     ring,
		config:   config,
		info: &pb.InfoResponse{
			ThisNode: config.ThisNode,
			Node:     config.Nodes,
		},
	}
	pb.RegisterRemoteFSServer(s, fs)
	cpb.RegisterElanServer(s, fs)
	return s, fs, lis
}

// A Server allows some maintenance operations on the gRPC server.
type Server interface {
	// GetClusterInfo returns diagnostic information about the cluster.
	GetClusterInfo() *cpb.ClusterInfoResponse
	// ListenUpdates receives updates from the given channel.
	ListenUpdates(<-chan *pb.Node)
}

type server struct {
	name     string
	storage  storage.Storage
	info     *pb.InfoResponse
	config   *cpb.Config
	ring     *cluster.Ring
	fan      Fan
	replicas int
}

func (s *server) Info(req *pb.InfoRequest, stream pb.RemoteFS_InfoServer) error {
	// Always send one message immediately
	if err := stream.Send(s.info); err != nil {
		return err
	}
	ch := s.fan.Add()
	defer s.fan.Remove(ch)
	for resp := range ch {
		if err := stream.Send(resp); err != nil {
			log.Warning("Error sending response: %s", err)
			return err
		}
	}
	return nil
}

func (s *server) Get(req *pb.GetRequest, stream pb.RemoteFS_GetServer) error {
	err := s.Retrieve(req, stream)
	if err != nil && status.Code(err) == codes.NotFound {
		// Check the replicas, they might have it instead.
		// This could happen because we're a replica but don't have the file (although
		// obvs that is nonideal), but also if we weren't a replica and the client sent
		// the request to the wrong place (which it shouldn't do ideally either).
		names, clients := s.ring.FindReplicas(req.Hash, s.replicas, s.name)
		for i, client := range clients {
			log.Warning("Failed to retrieve %s / %x, checking %s for it", req.Name, req.Hash, names[i])
			stream2, err := client.Retrieve(context.Background(), &pb.GetRequest{Hash: req.Hash, Name: req.Name, ChunkSize: req.ChunkSize})
			if err != nil {
				log.Warning("Failed to retrieve %s / %x: %s", req.Name, req.Hash, err)
				continue
			}
			// Stream all the responses back
			for {
				if resp, err := stream2.Recv(); err != nil {
					if err == io.EOF {
						return nil
					}
					return err
				} else if err := stream.Send(resp); err != nil {
					return err
				}
			}
		}
	}
	return err
}

func (s *server) Retrieve(req *pb.GetRequest, stream cpb.Elan_RetrieveServer) error {
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
		n, err := r.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if n != int(req.ChunkSize) {
			// Probably we have reached the end of the stream. Size the buffer appropriately.
			buf = buf[:n]
		}
		if err := stream.Send(&pb.GetResponse{Chunk: buf}); err != nil {
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
	var channels []chan *pb.PutRequest

	// Read one message to get the metadata
	req, err := stream.Recv()
	if err != nil {
		return err
	}
	w, err := s.storage.Save(req.Hash, req.Name)
	if err != nil {
		if os.IsExist(err) {
			return status.Errorf(codes.AlreadyExists, "%s / %x already exists", req.Name, req.Hash)
		}
		return err
	}
	defer w.Close()
	if _, err := w.Write(req.Chunk); err != nil {
		return err
	}
	if replicate {
		names, clients := s.ring.FindReplicas(req.Hash, s.replicas-1, s.name)
		log.Info("Replicating artifact %x to nodes %s", req.Hash, strings.Join(names, ", "))
		channels = make([]chan *pb.PutRequest, len(clients))
		for i, client := range clients {
			channels[i] = make(chan *pb.PutRequest, 1000)
			go s.forwardMessages(channels[i], client, names[i])
			channels[i] <- req
		}
	}
	// Now read & return the rest of the message.
	for {
		req, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		} else if req.Cancel {
			log.Warning("Client cancelled request to store %s", req.Name)
			w.Cancel()
			return status.Errorf(codes.Canceled, "client cancelled")
		} else if _, err := w.Write(req.Chunk); err != nil {
			return err
		}
		for _, ch := range channels {
			ch <- req
		}
	}
	for _, ch := range channels {
		close(ch)
	}
	return stream.SendAndClose(&pb.PutResponse{})
}

// forwardMessages forwards writes for replication from a channel to a gRPC client.
func (s *server) forwardMessages(ch <-chan *pb.PutRequest, client cpb.ElanClient, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	stream, err := client.Replicate(ctx)
	if err != nil {
		log.Error("Error replicating to %s", name)
		// Discard everything else in the channel.
		for range ch {
		}
		return
	}
	for req := range ch {
		if err := stream.Send(req); err != nil {
			log.Error("Error replicating to %s", name)
			for range ch {
			}
			return
		}
	}
	if _, err := stream.CloseAndRecv(); err != nil {
		if !grpcutil.IsAlreadyExists(err) {
			log.Error("Error receiving replication response: %s", err)
		}
	}
}

func (s *server) ListenUpdates(ch <-chan *pb.Node) {
	for node := range ch {
		s.updateNode(node)
		s.fan.Broadcast(s.info)
		s.config.Nodes = s.info.Node
		if err := s.storage.SaveConfig(s.config); err != nil {
			log.Error("Failed to save config: %s", err)
		}
	}
}

func (s *server) updateNode(node *pb.Node) {
	for _, n := range s.info.Node {
		if n.Name == node.Name {
			*n = *node
			return
		}
	}
	s.info.Node = append(s.info.Node, node)
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
