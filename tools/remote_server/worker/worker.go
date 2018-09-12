package worker

import (
	"context"
	"io"
	"sync"

	"github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"
	"gopkg.in/op/go-logging.v1"

	"grpcutil"
	pb "test/proto/remote"
)

var log = logging.MustGetLogger("worker")

// Connect connects to the master and receives messages.
// It continues forever until the server terminates.
func Connect(master, name, url string, port int) {
	// Start the local gRPC server first.
	w := &worker{}
	go w.start(port)

	conn, err := grpc.Dial(master, grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(grpc_retry.WithMax(3))))
	if err != nil {
		log.Fatalf("Failed to dial server: %s", err)
	}
	client := pb.NewRemoteTestMasterClient(conn)
	stream, err := client.ConnectWorker(context.Background())
	if err != nil {
		log.Fatalf("Failed to connect: %s", err)
	}
	err = stream.Send(&pb.ConnectWorkerRequest{
		Name: name,
		Url:  url,
	})
	if err != nil {
		log.Fatalf("Failed to connect: %s", err)
	}
	worker.Client = client

}

type worker struct {
	Client pb.RemoteTestMasterClient
}

// Start starts serving the worker gRPC server on the given port.
func (w *worker) Start(port int) {
	s := grpc.NewServer()
	pb.RegisterRemoteTestWorkerServer(s, w)
	grpcutil.StartServer(s, port)
}
