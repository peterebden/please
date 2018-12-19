// +build !bootstrap

package test

import (
	"context"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"

	"github.com/thought-machine/please/src/core"
	pb "github.com/thought-machine/please/src/test/proto/remote"
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
	rule := &pb.BuildLabel{
		PackageName: target.Label.PackageName,
		Name:        target.Label.Name,
	}
	response, err := remoteClient.GetTestWorker(ctx, &pb.TestWorkerRequest{
		Rule:   rule,
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
	outputs, err := loadTargetFiles(target.FullOutputs(), target.OutDir())
	if err != nil {
		return core.TestSuite{}, err
	}
	data, err := loadTargetFiles(target.AllData(state.Graph), "")
	if err != nil {
		return core.TestSuite{}, err
	}
	req := &pb.RemoteTestRequest{
		Rule:     rule,
		Outputs:  outputs,
		Data:     data,
		Command:  target.GetTestCommand(state),
		Coverage: state.NeedCoverage,
		Timeout:  int32(timeout / time.Second),
	}
	ctx, cancel = context.WithTimeout(context.Background(), timeout)
	defer cancel()
	startTime := time.Now()
	resp, err := client.ExecuteTest(ctx, req)
	duration := time.Since(startTime)
	if err != nil {
		return core.TestSuite{}, err
	}
	suite, err := parseAllTestResultContents(resp.Results)
	suite.Package = strings.Replace(target.Label.PackageName, "/", ".", -1)
	suite.Name = target.Label.Name
	suite.Duration = duration
	return suite, err
}

// testTimeout returns the timeout duration for a test, falling back to the config if it doesn't have one set.
func testTimeout(config *core.Configuration, target *core.BuildTarget) time.Duration {
	if target.TestTimeout != 0 {
		return target.TestTimeout
	}
	return time.Duration(config.Test.Timeout)
}

// loadTargetFiles loads a set of files for a target into the given map.
func loadTargetFiles(files []string, strip string) (map[string][]byte, error) {
	ret := map[string][]byte{}
	for _, file := range files {
		b, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, err
		} else if strip != "" {
			file = strings.TrimLeft(strings.TrimPrefix(file, strip), "/")
		}
		ret[file] = b
	}
	return ret, nil
}
