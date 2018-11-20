package grpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	pb "src/remote/proto/fs"
	//cpb "tools/elan/proto/cluster"
)

const address = "localhost:9928"

func TestRingInit(t *testing.T) {
	r := NewRing()
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
	r := NewRing()
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
	r := NewRing()
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
	r := NewRing()
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
