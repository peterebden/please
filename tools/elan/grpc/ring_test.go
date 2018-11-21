package grpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	pb "src/remote/proto/fs"
	cpb "tools/elan/proto/cluster"
)

const address = "localhost:9928"

// testClientFactory creates clients with no backing channel for testing.
// We need them to be unique for some tests, but we never use the channel.
func testClientFactory(address string) cpb.ElanClient {
	return cpb.NewElanClient(nil)
}

func TestRingInit(t *testing.T) {
	r := newRing(testClientFactory)
	err := r.Add("test1", address, nil)
	assert.NoError(t, err)
	nodes := r.Export()
	assert.Equal(t, 1, len(nodes))
	assert.Equal(t, "test1", nodes[0].Name)
	assert.Equal(t, address, nodes[0].Address)
	// We can't assert much about the tokens it's been given since they're randomised,
	// but the first one must always begin at 0.
	assert.EqualValues(t, 0, nodes[0].Ranges[0].Start)
}

func TestCannotReAddSameNode(t *testing.T) {
	r := newRing(testClientFactory)
	err := r.Add("test1", address, nil)
	assert.NoError(t, err)
	nodes := r.Export()
	err = r.Add("test1", address, nil)
	assert.Error(t, err)
	assert.Equal(t, nodes, r.Export())
}

func TestUpdateRejectsHashChanges(t *testing.T) {
	nodes := []*pb.Node{
		{
			Address: address,
			Name:    "node-1",
			Ranges: []*pb.Range{
				{Start: 0, End: 4611686018427387904},
				{Start: 4611686018427387905, End: 9223372036854775808},
				{Start: 9223372036854775809, End: 13835058055282163712},
				{Start: 13835058055282163713, End: 18446744073709551615},
			},
		},
	}
	r := newRing(testClientFactory)
	assert.NoError(t, r.Update(nodes))
	assert.Equal(t, nodes, r.Export())

	// This change is OK; ends of ranges are allowed to move
	nodes[0].Ranges[0].End = 17
	assert.NoError(t, r.Update(nodes))

	// This is not; a new node is claiming existing ranges.
	nodes[0].Name = "node-2"
	assert.Error(t, r.Update(nodes))
}

func TestUpdateAddingNewNodes(t *testing.T) {
	nodes := []*pb.Node{
		{
			Address: address,
			Name:    "node-1",
			Ranges: []*pb.Range{
				{Start: 0, End: 4611686018427387904},
				{Start: 4611686018427387905, End: 9223372036854775808},
				{Start: 9223372036854775809, End: 13835058055282163712},
				{Start: 13835058055282163713, End: 18446744073709551615},
			},
		},
	}
	r := newRing(testClientFactory)
	assert.NoError(t, r.Update(nodes))
	assert.Equal(t, nodes, r.Export())

	nodes = []*pb.Node{
		{
			Address: address,
			Name:    "node-1",
			Ranges: []*pb.Range{
				{Start: 0, End: 2305843009213693951},
				{Start: 4611686018427387905, End: 6917529027641081855},
				{Start: 9223372036854775809, End: 11529215046068469760},
				{Start: 13835058055282163713, End: 16140901064495857664},
			},
		}, {
			Address: address,
			Name:    "node-2",
			Ranges: []*pb.Range{
				{Start: 2305843009213693952, End: 4611686018427387904},
				{Start: 6917529027641081856, End: 9223372036854775808},
				{Start: 11529215046068469761, End: 13835058055282163712},
				{Start: 16140901064495857665, End: 18446744073709551615},
			},
		},
	}
	assert.NoError(t, r.Update(nodes))
	assert.Equal(t, nodes, r.Export())
}

func TestVerify(t *testing.T) {
	nodes := []*pb.Node{
		{
			Address: address,
			Name:    "node-1",
			Ranges: []*pb.Range{
				{Start: 0, End: 4611686018427387904},
				{Start: 4611686018427387905, End: 9223372036854775808},
				{Start: 9223372036854775809, End: 13835058055282163712},
				{Start: 13835058055282163713, End: 18446744073709551615},
			},
		},
	}
	r := newRing(testClientFactory)
	assert.NoError(t, r.Update(nodes))
	assert.NoError(t, r.Verify())
}

func TestVerify2(t *testing.T) {
	nodes := []*pb.Node{
		{
			Address: address,
			Name:    "node-1",
			Ranges: []*pb.Range{
				{Start: 0, End: 2305843009213693951},
				{Start: 4611686018427387905, End: 6917529027641081855},
				{Start: 9223372036854775809, End: 11529215046068469760},
				{Start: 13835058055282163713, End: 16140901064495857664},
			},
		}, {
			Address: address,
			Name:    "node-2",
			Ranges: []*pb.Range{
				{Start: 2305843009213693952, End: 4611686018427387904},
				{Start: 6917529027641081856, End: 9223372036854775808},
				{Start: 11529215046068469761, End: 13835058055282163712},
				{Start: 16140901064495857665, End: 18446744073709551615},
			},
		},
	}
	r := newRing(testClientFactory)
	assert.NoError(t, r.Update(nodes))
	assert.NoError(t, r.Verify())
}

func TestVerifyGap(t *testing.T) {
	nodes := []*pb.Node{
		{
			Address: address,
			Name:    "node-1",
			Ranges: []*pb.Range{
				{Start: 0, End: 2305843009213693951},
				{Start: 4611686018427387905, End: 6917529027641081853},
				{Start: 9223372036854775809, End: 11529215046068469760},
				{Start: 13835058055282163713, End: 16140901064495857664},
			},
		}, {
			Address: address,
			Name:    "node-2",
			Ranges: []*pb.Range{
				{Start: 2305843009213693952, End: 4611686018427387904},
				{Start: 6917529027641081856, End: 9223372036854775808},
				{Start: 11529215046068469761, End: 13835058055282163712},
				{Start: 16140901064495857665, End: 18446744073709551615},
			},
		},
	}
	r := newRing(testClientFactory)
	assert.NoError(t, r.Update(nodes))
	assert.Error(t, r.Verify())
}

func TestVerifyOverlap(t *testing.T) {
	nodes := []*pb.Node{
		{
			Address: address,
			Name:    "node-1",
			Ranges: []*pb.Range{
				{Start: 0, End: 2305843009213693951},
				{Start: 4611686018427387905, End: 6917529027641081857},
				{Start: 9223372036854775809, End: 11529215046068469760},
				{Start: 13835058055282163713, End: 16140901064495857664},
			},
		}, {
			Address: address,
			Name:    "node-2",
			Ranges: []*pb.Range{
				{Start: 2305843009213693952, End: 4611686018427387904},
				{Start: 6917529027641081856, End: 9223372036854775808},
				{Start: 11529215046068469761, End: 13835058055282163712},
				{Start: 16140901064495857665, End: 18446744073709551615},
			},
		},
	}
	r := newRing(testClientFactory)
	assert.NoError(t, r.Update(nodes))
	assert.Error(t, r.Verify())
}

func TestVerifyDoesNotStartAtZero(t *testing.T) {
	nodes := []*pb.Node{
		{
			Address: address,
			Name:    "node-1",
			Ranges: []*pb.Range{
				{Start: 2, End: 4611686018427387904},
				{Start: 4611686018427387905, End: 9223372036854775808},
				{Start: 9223372036854775809, End: 13835058055282163712},
				{Start: 13835058055282163713, End: 18446744073709551615},
			},
		},
	}
	r := newRing(testClientFactory)
	assert.NoError(t, r.Update(nodes))
	assert.Error(t, r.Verify())
}

func TestVerifyDoesNotEndAtMax(t *testing.T) {
	nodes := []*pb.Node{
		{
			Address: address,
			Name:    "node-1",
			Ranges: []*pb.Range{
				{Start: 0, End: 4611686018427387904},
				{Start: 4611686018427387905, End: 9223372036854775808},
				{Start: 9223372036854775809, End: 13835058055282163712},
				{Start: 13835058055282163713, End: 18446744073709551613},
			},
		},
	}
	r := newRing(testClientFactory)
	assert.NoError(t, r.Update(nodes))
	assert.Error(t, r.Verify())
}

func TestVerifyEmptyRing(t *testing.T) {
	r := newRing(testClientFactory)
	assert.Error(t, r.Verify())
}

func TestFind(t *testing.T) {
	nodes := []*pb.Node{
		{
			Address: address,
			Name:    "node-1",
			Ranges: []*pb.Range{
				{Start: 0, End: 2305843009213693951},
				{Start: 4611686018427387905, End: 6917529027641081857},
				{Start: 9223372036854775809, End: 11529215046068469760},
				{Start: 13835058055282163713, End: 16140901064495857664},
			},
		}, {
			Address: address,
			Name:    "node-2",
			Ranges: []*pb.Range{
				{Start: 2305843009213693952, End: 4611686018427387904},
				{Start: 6917529027641081856, End: 9223372036854775808},
				{Start: 11529215046068469761, End: 13835058055282163712},
				{Start: 16140901064495857665, End: 18446744073709551615},
			},
		},
	}
	r := newRing(testClientFactory)
	assert.NoError(t, r.Update(nodes))

	name, client1 := r.Find(0)
	assert.Equal(t, "node-1", name)

	name, client2 := r.Find(6917529027841081856)
	assert.Equal(t, "node-2", name)

	name, client3 := r.Find(ringMax)
	assert.Equal(t, "node-2", name)
	assert.Equal(t, client1, client3)

	clients := r.FindN(0, 3)
	assert.EqualValues(t, []cpb.ElanClient{client1, client2, client1}, clients)

	clients = r.FindN(6917529027841081856, 3)
	assert.EqualValues(t, []cpb.ElanClient{client2, client1, client2}, clients)

	clients = r.FindN(ringMax, 3)
	assert.EqualValues(t, []cpb.ElanClient{client2, client1, client2}, clients)
}

func TestMerge(t *testing.T) {
	nodes := []*pb.Node{
		{
			Address: address,
			Name:    "node-1",
			Ranges: []*pb.Range{
				{Start: 0, End: 2305843009213693951},
				{Start: 4611686018427387905, End: 6917529027641081857},
				{Start: 9223372036854775809, End: 11529215046068469760},
				{Start: 13835058055282163713, End: 16140901064495857664},
			},
		}, {
			Address: address,
			Name:    "node-2",
			Ranges: []*pb.Range{
				{Start: 2305843009213693952, End: 4611686018427387904},
				{Start: 6917529027641081856, End: 9223372036854775808},
				{Start: 11529215046068469761, End: 13835058055282163712},
				{Start: 16140901064495857665, End: 18446744073709551615},
			},
		},
	}
	r := newRing(testClientFactory)
	assert.NoError(t, r.Update(nodes))

	// Merging existing nodes should have no effect
	assert.NoError(t, r.Merge(nodes[0].Name, nodes[0].Address, nodes[0].Ranges))
	assert.NoError(t, r.Merge(nodes[1].Name, nodes[1].Address, nodes[1].Ranges))
	assert.Equal(t, nodes, r.Export())

	// Merge a new node. This simulates a node coming back up that we don't know about;
	// it shouldn't really happen very often but might if the ring was rebuilding itself.
	nodes = append(nodes, &pb.Node{
		Address: address,
		Name:    "node-3",
		Ranges: []*pb.Range{
			{Start: 3005843009213693952, End: 4611686018427387904},
			{Start: 7017529027641081856, End: 9223372036854775808},
		},
	})
	assert.NoError(t, r.Merge(nodes[2].Name, nodes[2].Address, nodes[2].Ranges))
	// The existing ranges should have updated correctly.
	nodes[1].Ranges[0].End = 3005843009213693951
	nodes[1].Ranges[1].End = 7017529027641081855
	assert.Equal(t, nodes, r.Export())
}
