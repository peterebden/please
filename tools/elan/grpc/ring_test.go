package grpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
