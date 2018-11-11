// Package grpcutil implements several useful utility functions for gRPC.
package grpcutil

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"gopkg.in/op/go-logging.v1"
)

var log = logging.MustGetLogger("grpcutil")

// StartServer starts a server on the given port.
// It runs forever until terminated.
// Signals will be automatically handled using HandleSignals.
func StartServer(s *grpc.Server, port int) {
	s.Serve(SetupServer(s, port))
}

// SetupServer creates a server & opens the given port for listening, but does not start running
// it. Call s.Serve on the returned listener to set it going.
// This is often useful for testing where we want to know the port is open before continuing.
func SetupServer(s *grpc.Server, port int) net.Listener {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("%s", err)
	}
	go HandleSignals(s)
	log.Notice("Serving on port %d", port)
	return lis
}

// HandleSignals received SIGTERM / SIGINT etc to gracefully shut down a gRPC server.
// Repeated signals cause the server to terminate at increasing levels of urgency.
// N.B. This function never returns, so you would typically run it in a new goroutine.
func HandleSignals(s *grpc.Server) {
	c := make(chan os.Signal, 3) // Channel should be buffered a bit
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	sig := <-c
	log.Warning("Received signal %s, gracefully shutting down gRPC server", sig)
	go s.GracefulStop()
	sig = <-c
	log.Warning("Received signal %s, non-gracefully shutting down gRPC server", sig)
	go s.Stop()
	sig = <-c
	log.Fatalf("Received signal %s, terminating\n", sig)
}

// Dial wraps grpc.Dial with some default options.
// Errors are not returned; in practice they basically never happen since we don't pass WithBlock().
func Dial(url string) *grpc.ClientConn {
	conn, err := grpc.Dial(url, grpc.WithInsecure(), grpc.WithTimeout(5*time.Second))
	if err != nil {
		log.Fatalf("Failed to dial remote server: %s", err)
	}
	return conn
}
