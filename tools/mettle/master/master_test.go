package master

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/thought-machine/please/src/grpcutil"
	pb "github.com/thought-machine/please/src/remote/proto/remote"
	wpb "github.com/thought-machine/please/tools/mettle/proto/worker"
)

const port = 9922
const url = "127.0.0.1:9922"

func TestNoWorkers(t *testing.T) {
	s, _, lis := createServer(port, 0, time.Nanosecond)
	defer s.Stop()
	go s.Serve(lis)

	client := pb.NewRemoteWorkerClient(grpcutil.Dial(url))
	stream, err := client.RemoteTask(context.Background())
	require.NoError(t, err)
	err = stream.Send(&pb.RemoteTaskRequest{Target: "test"})
	assert.NoError(t, err)
	_, err = stream.Recv()
	assert.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, grpc.Code(err))
}

func TestWithWorker(t *testing.T) {
	s, _, lis := createServer(port, 3, time.Second)
	defer s.Stop()
	go s.Serve(lis)
	runFakeWorker(t)

	client := pb.NewRemoteWorkerClient(grpcutil.Dial(url))
	stream, err := client.RemoteTask(context.Background())
	require.NoError(t, err)
	err = stream.Send(&pb.RemoteTaskRequest{Target: "test", Command: "true"})
	assert.NoError(t, err)

	// The first message we get back should just tell us that it's building.
	resp, err := stream.Recv()
	assert.NoError(t, err)
	assert.False(t, resp.Complete)
	assert.True(t, resp.Success)

	// The next one should be the actual result
	resp, err = stream.Recv()
	assert.NoError(t, err)
	assert.True(t, resp.Complete)
	assert.True(t, resp.Success)
}

func runFakeWorker(t *testing.T) {
	conn := grpcutil.Dial(url)
	client := wpb.NewRemoteMasterClient(conn)
	stream, err := client.Work(context.Background())
	require.NoError(t, err)
	log.Notice("sending registration request")
	err = stream.Send(&wpb.WorkRequest{Name: "test"})
	require.NoError(t, err)
	go func() {
		for {
			resp, err := stream.Recv()
			require.NoError(t, err)
			log.Notice("received message to worker")
			if resp.Shutdown {
				break
			}
			log.Notice("sending worker response")
			err = stream.Send(&wpb.WorkRequest{Response: &pb.RemoteTaskResponse{
				Success:  resp.Request.Command == "true",
				Complete: true,
			}})
			require.NoError(t, err)
		}
		log.Notice("Shutting down worker")
		stream.CloseSend()
	}()
}
