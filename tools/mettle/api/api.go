// Package api implements the remote execution API server.
package api

import (
	"context"
	"fmt"
	"net"

	pb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"gocloud.dev/pubsub"
	bs "google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/genproto/googleapis/longrunning"
	"gopkg.in/op/go-logging.v1"
)

var log = logging.MustGetLogger("api")

// ServeForever serves on the given port until terminated.
func ServeForever(port int, queue, storage string) {
	if err := serveForever(port, queue, storage); err != nil {
		log.Fatalf("%s", err)
	}
}

func serveForever(port int, queue, storage string) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("Failed to listen on %s: %v", lis.Addr(), err)
	}
	sub, err := pubsub.OpenSubscription(context.Background(), queue)
	if err != nil {
		return fmt.Errorf("Failed to open connection to queue: %s", err)
	}
	srv := &server{
		sub: sub,
	}

}
