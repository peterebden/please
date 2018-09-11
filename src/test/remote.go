// +build !bootstrap

package test

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc"

	"core"
	pb "test/proto/remote"
)

var remoteClient pb.RemoteTestMasterClient
var remoteClientOnce sync.Once

func runTestRemotely(tid int, state *core.BuildState, target *core.BuildTarget) ([]byte, error) {
	state.LogBuildResult(tid, target.Label, core.TargetTesting, "Contacting remote server...")
	remoteClientOnce.Do(func() {
		// TODO(peterebden): Add TLS support (as always)
		conn, err := grpc.Dial(state.Config.Test.RemoteURL.String(), grpc.WithTimeout(10*time.Second), grpc.WithInsecure())
		if err != nil {
			// It's not very nice to die here, but in practice this very rarely happens since we
			// didn't pass WithBlock(), so most errors are picked up by the RPC call below.
			log.Fatalf("Failed to dial remote test server: %s", err)
		}
		remoteClient = pb.NewRemoteTestMasterClient(conn)
	})
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout(state.Config, target))
	defer cancel()
	response, err := remoteClient.GetTestWorker(ctx, &pb.TestWorkerRequest{
		Rule:   target.Label.String(),
		Labels: target.Labels,
	})
	if err != nil {
		log.Error("Failed to contact remote worker server: %s", err)
		return nil, err
	}
	response = response
	panic("didn't expect to get here")
}

// testTimeout returns the timeout duration for a test, falling back to the config if it doesn't have one set.
func testTimeout(config *core.Configuration, target *core.BuildTarget) time.Duration {
	if target.TestTimeout != 0 {
		return target.TestTimeout
	}
	return time.Duration(config.Test.Timeout)
}
