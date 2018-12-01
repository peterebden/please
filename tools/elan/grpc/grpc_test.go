package grpc

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"grpcutil"
	pb "src/remote/proto/fs"
	cpb "tools/elan/proto/cluster"
	"tools/elan/storage"
)

func TestServer(t *testing.T) {
	// Initialise the first server
	store1, err := storage.Init("test_store_1", 1024)
	require.NoError(t, err)
	s1, srv1, lis1 := start(0, nil, store1, "test-1", "", 2)
	go s1.Serve(lis1)
	defer s1.Stop()

	// Initialise a second server to talk to it
	store2, err := storage.Init("test_store_2", 1024)
	require.NoError(t, err)
	s2, _, lis2 := start(0, []string{lis1.Addr().String()}, store2, "test-2", "", 2)
	go s2.Serve(lis2)
	defer s2.Stop()

	// Verify things look right
	resp, err := srv1.ClusterInfo(context.Background(), &cpb.ClusterInfoRequest{})
	require.NoError(t, err)
	assert.True(t, resp.Healthy)
	assert.Equal(t, 2, len(resp.Nodes))
	assert.Equal(t, "test-1", resp.Nodes[0].Name)
	assert.Equal(t, "test-2", resp.Nodes[1].Name)

	// Simulate node 2 rejoining the cluster
	resp2, err := srv1.Register(context.Background(), &cpb.RegisterRequest{
		Node: resp.Nodes[1],
	})
	require.NoError(t, err)
	assert.True(t, resp2.Accepted)

	// Simulate it rejoining again after forgetting everything (maybe it's been wiped)
	resp2, err = srv1.Register(context.Background(), &cpb.RegisterRequest{
		Node: &pb.Node{
			Name:    "test-2",
			Address: lis2.Addr().String(),
		},
	})
	require.NoError(t, err)
	assert.True(t, resp2.Accepted)

	// Simulate it rejoining with different tokens
	resp2, err = srv1.Register(context.Background(), &cpb.RegisterRequest{
		Node: &pb.Node{
			Name:    "test-2",
			Address: lis2.Addr().String(),
			Ranges: []*pb.Range{
				{Start: 1},
				{Start: 2},
				{Start: 3},
				{Start: 4},
			},
		},
	})
	require.NoError(t, err)
	assert.False(t, resp2.Accepted)

	// Now test actually storing something.
	client1 := pb.NewRemoteFSClient(grpcutil.Dial(lis1.Addr().String()))
	ps, err := client1.Put(context.Background())
	require.NoError(t, err)

	hash := uint64(12345)
	name := "test.txt"
	content := []byte("testing testing one two three")
	err = ps.Send(&pb.PutRequest{
		Hash:  hash,
		Name:  name,
		Chunk: content[:16],
	})
	assert.NoError(t, err)
	err = ps.Send(&pb.PutRequest{Chunk: content[16:]})
	assert.NoError(t, err)
	_, err = ps.CloseAndRecv()
	assert.NoError(t, err)

	// Now receive it back again
	gs, err := client1.Get(context.Background(), &pb.GetRequest{
		Hash: hash,
		Name: name,
	})
	require.NoError(t, err)
	gr, err := gs.Recv()
	require.NoError(t, err)
	assert.EqualValues(t, content, gr.Chunk)
	_, err = gs.Recv()
	assert.Equal(t, io.EOF, err)

	// And we should be able to receive it from the other node as well.
	client2 := pb.NewRemoteFSClient(grpcutil.Dial(lis2.Addr().String()))
	gs, err = client2.Get(context.Background(), &pb.GetRequest{
		Hash: hash,
		Name: name,
	})
	require.NoError(t, err)
	gr, err = gs.Recv()
	require.NoError(t, err)
	assert.EqualValues(t, content, gr.Chunk)
	_, err = gs.Recv()
	assert.Equal(t, io.EOF, err)

	// This tests failed replication & the behaviour when a node doesn't have a
	// copy of an artifact. It works because we never provide a working address for
	// node-1 so requests can't be replicated to it, but nodes should still forward
	// requests that they don't have content for locally.
	ps, err = client2.Put(context.Background())
	require.NoError(t, err)
	hash = 23456
	name = "test2.txt"
	err = ps.Send(&pb.PutRequest{
		Hash:  hash,
		Name:  name,
		Chunk: content,
	})
	assert.NoError(t, err)
	_, err = ps.CloseAndRecv()
	assert.NoError(t, err)

	gs, err = client1.Get(context.Background(), &pb.GetRequest{
		Hash: hash,
		Name: name,
	})
	require.NoError(t, err)
	gr, err = gs.Recv()
	require.NoError(t, err)
	assert.EqualValues(t, content, gr.Chunk)
	_, err = gs.Recv()
	assert.Equal(t, io.EOF, err)
}
