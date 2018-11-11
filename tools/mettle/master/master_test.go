package master

import (
	"context"
	"fmt"
	"testing"

	"github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"grpcutil"
	pb "test/proto/remote"
)

const port = 9922

func TestRegisterAndDeregister(t *testing.T) {
	s := createServer()
	defer s.Stop()
	go grpcutil.StartServer(s, port)

	client := createClient(t)
	stream, err := client.ConnectWorker(context.Background())
	require.NoError(t, err)
	err = stream.Send(&pb.ConnectWorkerRequest{
		Name: "bob",
		Url:  "localhost:9923",
	})
	require.NoError(t, err)

	resp, err := client.GetTestWorker(context.Background(), &pb.TestWorkerRequest{
		Rule: "//tools/remote_server/master:master_test",
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "bob", resp.Name)
	assert.Equal(t, "localhost:9923", resp.Url)
	assert.Equal(t, "", resp.Error)
}

func createClient(t *testing.T) pb.RemoteTestMasterClient {
	conn, err := grpc.Dial(fmt.Sprintf("127.0.0.1:%d", port), grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(grpc_retry.WithMax(3))))
	require.NoError(t, err)
	return pb.NewRemoteTestMasterClient(conn)
}
