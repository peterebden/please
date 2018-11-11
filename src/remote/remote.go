// +build !bootstrap

// Package remote is responsible for communication with remote build servers.
// Some of the nomenclature can be a little confusing since we use "remote" in other contexts
// (e.g. local worker servers that are "remote" to this process). Eventually we might clean
// that all up a bit to be a bit more consistent.
package remote

import (
	"context"
	"fmt"
	"sync"
	"time"

	retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"
	"gopkg.in/op/go-logging.v1"

	"core"
	pb "remote/proto/remote"
)

var log = logging.MustGetLogger("remote")

var remoteClient pb.RemoteWorkerClient
var remoteClientOnce sync.Once

const dialTimeout = 10 * time.Second

// Build causes a target to be built on a remote worker.
//
// N.B. It does *not* necessarily cause outputs to appear locally.
func Build(tid int, state *core.BuildState, target *core.BuildTarget, hash []byte) error {
	state.LogBuildResult(tid, target.Label, core.TargetBuilding, "Dispatching to remote worker...")
	remoteClientOnce.Do(func() { initClient(state) })

	timeout := core.TimeoutOrDefault(target.BuildTimeout, state.Config.Build.Timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, err := remoteClient.RemoteTask(ctx)
	if err != nil {
		return err
	}
	defer client.CloseSend()
	// Send initial request
	sources, err := convertSources(state, target)
	if err != nil {
		return err
	}
	prompt := target.PostBuildFunction != nil
	if err := client.Send(&pb.RemoteTaskRequest{
		Target:  target.Label.String(),
		Command: target.GetCommand(state),
		Hash:    hash,
		Outputs: target.Outputs(),
		Prompt:  prompt,
		Env:     core.StampedBuildEnvironment(state, target, hash, "${TMP_DIR}/"),
		Files:   sources,
	}); err != nil {
		return err
	}
	// Now read responses
	for {
		resp, err := client.Recv()
		if err != nil {
			return err // including EOF, we should usually break before that.
		} else if !resp.Success {
			return fmt.Errorf("%s", resp.Msg)
		} else if resp.Complete {
			if !prompt {
				state.LogBuildResult(tid, target.Label, core.TargetBuiltRemotely, "Built remotely")
				break // We are done.
			}
			// Target has a post-build function to be run, which we have to do.
			if err := state.Parser.RunPostBuildFunction(tid, state, target, string(resp.Output)); err != nil {
				return err
			}
			// Communicate the new state of the target back again.
			if err := client.Send(&pb.RemoteTaskRequest{Outputs: target.Outputs()}); err != nil {
				return err
			}
			prompt = false // we won't do this again
		} else {
			// Just a progress message.
			state.LogBuildResult(tid, target.Label, core.TargetBuilding, resp.Msg)
		}
	}
	return nil
}

// initClient sets up the remote client
func initClient(state *core.BuildState) {
	// TODO(peterebden): TLS, as usual...
	conn, err := grpc.Dial(
		state.Config.Build.RemoteURL.String(),
		grpc.WithTimeout(5*time.Second),
		grpc.WithInsecure(),
		grpc.WithStreamInterceptor(
			retry.StreamClientInterceptor(
				retry.WithMax(3),
				retry.WithBackoff(retry.BackoffLinear(2*time.Second)))))
	if err != nil {
		// It's not very nice to die here, but in practice this very rarely happens since we
		// didn't pass WithBlock(), so most errors are picked up by the actual RPC call.
		log.Fatalf("Failed to dial remote server: %s", err)
	}
	remoteClient = pb.NewRemoteWorkerClient(conn)
}

func convertSources(state *core.BuildState, target *core.BuildTarget) ([]*pb.Fileset, error) {
	ret := []*pb.Fileset{}
	for source := range core.IterSources(state.Graph, target) {
		hash, err := state.PathHasher.Hash(source.Src, false)
		if err != nil {
			return nil, err
		}
		ret = append(ret, &pb.Fileset{Hash: hash, Filenames: []string{source.Src}})
	}
	return ret, nil
}
