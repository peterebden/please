package fsclient

import (
	"bytes"
	"io"
	"io/ioutil"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"grpcutil"
	pb "src/remote/proto/fs"
)

func TestClient(t *testing.T) {
	s1 := grpc.NewServer()
	srv1 := newServer()
	pb.RegisterRemoteFSServer(s1, srv1)
	lis1 := grpcutil.SetupServer(s1, 0)
	go s1.Serve(lis1)

	s2 := grpc.NewServer()
	srv2 := newServer()
	pb.RegisterRemoteFSServer(s2, srv2)
	lis2 := grpcutil.SetupServer(s2, 0)
	go s2.Serve(lis2)

	info = &pb.InfoResponse{
		Node: []*pb.Node{
			{
				Address: lis1.Addr().String(),
				Name:    "node-1",
				Ranges: []*pb.Range{
					{Start: 0, End: 999},
					{Start: 1000, End: 1999},
					{Start: 2000, End: 2999},
					{Start: 3000, End: 3999},
					{Start: 4000, End: 4999},
					{Start: 5000, End: 5999},
					{Start: 6000, End: math.MaxUint64},
				},
			},
			{
				Address: lis2.Addr().String(),
				Name:    "node-2",
				Ranges: []*pb.Range{
					{Start: 0, End: 999},
					{Start: 1000, End: 1999},
					{Start: 2000, End: 2999},
					{Start: 3000, End: 3999},
					{Start: 4000, End: 4999},
					{Start: 5000, End: 5999},
					{Start: 6000, End: math.MaxUint64},
				},
			},
		},
	}
	info.ThisNode = info.Node[0]

	client := New([]string{lis1.Addr().String()})
	// Make this big enough so it exercises the chunking code properly.
	content := bytes.Repeat([]byte("testing"), 100000)
	hash := []byte("wevs")
	const name1 = "test1.txt"
	const name2 = "test2.txt"
	err := client.Put([]string{name1}, hash, []io.ReadSeeker{bytes.NewReader(content)})
	assert.NoError(t, err)
	err = client.Put([]string{name2}, hash, []io.ReadSeeker{bytes.NewReader(content)})
	assert.NoError(t, err)

	rs, err := client.Get([]string{name1, name2}, hash)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(rs))
	b1, err := ioutil.ReadAll(rs[0])
	assert.NoError(t, err)
	b2, err := ioutil.ReadAll(rs[1])
	assert.NoError(t, err)
	assert.EqualValues(t, content, b1)
	assert.EqualValues(t, content, b2)
}

// server implements a fake in-memory server for testing.
type server struct {
	files   map[key][]byte
	Updates chan *pb.InfoResponse
}

func newServer() *server {
	return &server{
		files:   map[key][]byte{},
		Updates: make(chan *pb.InfoResponse),
	}
}

type key struct {
	Hash uint64
	Name string
}

func (s *server) Info(req *pb.InfoRequest, stream pb.RemoteFS_InfoServer) error {
	stream.Send(info)
	for msg := range s.Updates {
		stream.Send(msg)
	}
	return nil
}

func (s *server) Get(req *pb.GetRequest, stream pb.RemoteFS_GetServer) error {
	f, present := s.files[key{Hash: req.Hash, Name: req.Name}]
	if !present {
		return status.Errorf(codes.NotFound, "file not found")
	}
	for {
		if int(req.ChunkSize) > len(f) {
			stream.Send(&pb.GetResponse{Chunk: f})
			break
		}
		stream.Send(&pb.GetResponse{Chunk: f[:req.ChunkSize]})
		f = f[req.ChunkSize:]
	}
	return nil
}

func (s *server) Put(stream pb.RemoteFS_PutServer) error {
	req, _ := stream.Recv()
	k := key{Hash: req.Hash, Name: req.Name}
	f := req.Chunk
	var err error
	for {
		req, err = stream.Recv()
		if err != nil {
			break
		}
		f = append(f, req.Chunk...)
	}
	s.files[k] = f
	return stream.SendAndClose(&pb.PutResponse{})
}

var info *pb.InfoResponse
