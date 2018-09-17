// +build !bootstrap

package test

import (
	"context"
	"sync"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"

	"core"
	pb "test/proto/remote"
)

var remoteClient pb.RemoteTestMasterClient
var remoteClientOnce sync.Once

const dialTimeout = 10 * time.Second

func runTestRemotely(tid int, state *core.BuildState, target *core.BuildTarget) (core.TestSuite, error) {
	state.LogBuildResult(tid, target.Label, core.TargetTesting, "Contacting remote server...")
	remoteClientOnce.Do(func() {
		// TODO(peterebden): Add TLS support (as always)
		conn, err := grpc.Dial(state.Config.Test.RemoteURL.String(), grpc.WithTimeout(dialTimeout), grpc.WithInsecure(),
			grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(grpc_retry.WithMax(3))))
		if err != nil {
			// It's not very nice to die here, but in practice this very rarely happens since we
			// didn't pass WithBlock(), so most errors are picked up by the RPC call below.
			log.Fatalf("Failed to dial remote test server: %s", err)
		}
		remoteClient = pb.NewRemoteTestMasterClient(conn)
	})
	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()
	response, err := remoteClient.GetTestWorker(ctx, &pb.TestWorkerRequest{
		Rule:   target.Label.String(),
		Labels: target.Labels,
	})
	if err != nil {
		log.Error("Failed to contact remote worker server: %s", err)
		return core.TestSuite{}, err
	}
	// Now dial up a gRPC channel to the worker.
	conn, err := grpc.Dial(response.Url, grpc.WithTimeout(dialTimeout), grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(grpc_retry.WithMax(3))))
	if err != nil {
		return core.TestSuite{}, err
	}
	client := pb.NewRemoteTestWorkerClient(conn)

	timeout := testTimeout(state.Config, target)
	ctx, cancel = context.WithTimeout(context.Background(), timeout)
	defer cancel()
	startTime := time.Now()
	resp, err := client.ExecuteTest(req)
	duration := time.Since(startTime)
	if err != nil {
		return core.TestSuite{}, err
	}
	// From here we've got a good test response, so don't return errors any more.
	return core.TestSuite{
		Package:    strings.Replace(target.Label.PackageName, "/", ".", -1),
		Name:       target.Label.Name,
		Duration:   duration,
		Properties: parsedSuite.Properties,
		TestCases:  parsedSuite.TestCases,
	}, nil
}

// testTimeout returns the timeout duration for a test, falling back to the config if it doesn't have one set.
func testTimeout(config *core.Configuration, target *core.BuildTarget) time.Duration {
	if target.TestTimeout != 0 {
		return target.TestTimeout
	}
	return time.Duration(config.Test.Timeout)
}
