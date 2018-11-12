package worker

import (
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"grpcutil"
	rpb "src/remote/proto/remote"
	pb "tools/mettle/proto/worker"
)

const port = 9923
const url = "127.0.0.1:9923"

func TestWorker(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	s, m, lis := createServer()
	go s.Serve(lis)
	go func() {
		Connect(url, "test", ".", mockClient{})
		wg.Done()
	}()
	// As noted in other places, the "requests" and "responses" are a bit confused here, since
	// the worker acts as a client.
	m.responses <- &pb.WorkResponse{Request: &rpb.RemoteTaskRequest{
		Target:  "//test",
		Command: "true",
	}}
	req <- m.requests
	assert.True(req.Success)
	assert.True(req.Complete)

	// Now shut the server down
	m.responses <- &pb.WorkResponse{Shutdown: true}
	wg.Wait() // This verifies that the Connect function finishes correctly
}

func createServer() (*grpc.Server, *master, net.Listener) {
	s := grpc.NewServer()
	m := &master{
		requests:  make(chan *pb.WorkRequest),
		responses: make(chan *pb.WorkResponse),
	}
	pb.RegisterRemoteMasterServer(s, m)
	return s, m, grpcutil.SetupServer(s, port)
}

type master struct {
	requests  chan *pb.WorkRequest
	responses chan *pb.WorkResponse
}

func (m *master) Work(stream pb.RemoteMaster_WorkServer) error {
	// Worker registration, do nothing
	stream.Recv()
	// Blast all the messages off to it
	go func() {
		for {
			if req, err := stream.Recv(); err != nil {
				log.Warning("unexpected error: %s", err)
				break
			} else {
				m.requests <- req
			}
		}
	}()
	for resp := range m.responses {
		stream.Send(resp)
	}
	return nil
}

// mockClient is used for testing, it does nothing.
// TODO(peterebden): we should test real file access at some point...
type mockClient struct{}

func (m mockClient) Get(filenames []string, hash []byte) ([]io.ReadCloser, error) {
	return nil, nil
}

func (m mockClient) Put(filename string, content io.Reader, hash []byte) error {
	return nil
}
