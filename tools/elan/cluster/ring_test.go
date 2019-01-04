package cluster

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	pb "github.com/thought-machine/please/src/remote/proto/fs"
	cpb "github.com/thought-machine/please/tools/elan/proto/cluster"
)

const address = "localhost:9928"

// testClientFactory creates clients with no backing channel for testing.
// We need them to be unique for some tests, but we never use the channel.
func testClientFactory(address string) cpb.ElanClient {
	return cpb.NewElanClient(nil)
}

func TestRingInit(t *testing.T) {
	r := newRing(testClientFactory)
	err := r.Add(&pb.Node{Name: "test1", Address: address})
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
	err := r.Add(&pb.Node{Name: "test1", Address: address})
	assert.NoError(t, err)
	nodes := r.Export()
	err = r.Add(&pb.Node{Name: "test1", Address: address})
	assert.Error(t, err)
	assert.Equal(t, nodes, r.Export())
}

func TestUpdateRejectsHashChanges(t *testing.T) {
	node := &pb.Node{
		Address: address,
		Name:    "node-1",
		Ranges: []*pb.Range{
			{Start: 0, End: 4611686018427387904},
			{Start: 4611686018427387905, End: 9223372036854775808},
			{Start: 9223372036854775809, End: 13835058055282163712},
			{Start: 13835058055282163713, End: 18446744073709551615},
		},
	}
	r := newRing(testClientFactory)
	assert.NoError(t, update(r, node))
	assert.EqualValues(t, node, r.Export()[0])

	// This change is OK; ends of ranges are allowed to move
	node.Ranges[0].End = 17
	assert.NoError(t, update(r, node))

	// This is not; a new node is claiming existing ranges.
	node = &pb.Node{
		Address: address,
		Name:    "node-2",
		Ranges: []*pb.Range{
			{Start: 0, End: 4611686018427387904},
		},
	}
	assert.Error(t, update(r, node))
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
	assert.NoError(t, update(r, nodes[0]))
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
	assert.NoError(t, update(r, nodes[1]))
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
	assert.NoError(t, update(r, nodes[0]))
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
	assert.NoError(t, update(r, nodes[0]))
	assert.NoError(t, update(r, nodes[1]))
	assert.NoError(t, r.Verify())
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
	assert.NoError(t, update(r, nodes[0]))
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
	assert.NoError(t, update(r, nodes[0]))
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
	assert.NoError(t, update(r, nodes[0]))
	assert.NoError(t, update(r, nodes[1]))

	name, client1 := r.Find(0)
	assert.Equal(t, "node-1", name)

	name, client2 := r.Find(6917529027841081856)
	assert.Equal(t, "node-2", name)

	name, client3 := r.Find(ringMax)
	assert.Equal(t, "node-2", name)
	assert.Equal(t, client1, client3)

	names, clients := r.FindReplicas(0, 1, "node-1")
	assert.EqualValues(t, []string{"node-2"}, names)
	assert.EqualValues(t, []cpb.ElanClient{client2}, clients)

	names, clients = r.FindReplicas(0, 1, "node-2")
	assert.EqualValues(t, []string{"node-1"}, names)
	assert.EqualValues(t, []cpb.ElanClient{client1}, clients)

	names, clients = r.FindReplicas(2305843009213693952, 1, "node-1")
	assert.EqualValues(t, []string{"node-2"}, names)
	assert.EqualValues(t, []cpb.ElanClient{client2}, clients)
}

func TestGenToken(t *testing.T) {
	r := newRing(testClientFactory)
	for i := 0; i < 100; i++ {
		for tok := 0; tok < numTokens; tok++ {
			r.genToken(uint64(tok), fmt.Sprintf("test-%d", i), nil)
		}
	}
}

func update(r *Ring, node *pb.Node) error {
	_, err := r.Update(node)
	return err
}
