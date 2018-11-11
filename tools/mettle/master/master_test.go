package master

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"grpcutil"
	pb "remote/proto/remote"
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
