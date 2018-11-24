package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "src/remote/proto/fs"
	cpb "tools/elan/proto/cluster"
	"tools/elan/storage"
)

func TestServer(t *testing.T) {
	// Initialise the first server
	store1, err := storage.Init("test_store_1", 1024)
	require.NoError(t, err)
	s1, srv1, lis1 := start(0, nil, store1, "test-1", "")
	go s1.Serve(lis1)
	defer s1.Stop()

	// Initialise a second server to talk to it
	store2, err := storage.Init("test_store_2", 1024)
	require.NoError(t, err)
	s2, _, lis2 := start(0, []string{lis1.Addr().String()}, store2, "test-2", "")
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
}
