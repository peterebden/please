package grpc

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pb "github.com/thought-machine/please/src/remote/proto/fs"
)

func TestFanAdd(t *testing.T) {
	var f Fan
	ch1 := f.Add()
	ch2 := f.Add()
	f.Broadcast(&pb.InfoResponse{ThisNode: &pb.Node{Name: "test"}})
	msg := <-ch1
	assert.Equal(t, "test", msg.ThisNode.Name)
	msg = <-ch2
	assert.Equal(t, "test", msg.ThisNode.Name)
}

func TestFanRemove(t *testing.T) {
	var f Fan
	ch1 := f.Add()
	ch2 := f.Add()
	f.Remove(ch1)
	f.Broadcast(&pb.InfoResponse{ThisNode: &pb.Node{Name: "test"}})
	msg := <-ch2
	assert.Equal(t, "test", msg.ThisNode.Name)
	_, ok := <-ch1
	assert.False(t, ok)
}
