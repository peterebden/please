// +build proto

package test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"core"
	pb "test/proto/worker"
)

const remoteConnTimeout = 2 * time.Second

type remoteWorker struct {
	client pb.TestWorkerClient
	id     int
}

// RunRemotely runs a single test on a remote worker.
func (worker *remoteWorker) RunRemotely(state *core.BuildState, target *core.BuildTarget) {
	client, err := manager.GetClient(state.Config)
	if err != nil {
		return nil, nil, nil, err
	}
	timeout := target.TestTimeout
	if timeout == 0 {
		timeout = time.Duration(state.Config.Test.Timeout)
	}
	request := pb.TestRequest{
		Rule:     &pb.BuildLabel{PackageName: target.Label.PackageName, Name: target.Label.Name},
		Command:  target.GetTestCommand(),
		Coverage: state.NeedCoverage,
		TestName: state.TestArgs,
		Timeout:  int32(timeout.Seconds()),
		Labels:   target.Labels,
		NoOutput: target.NoTestOutput,
		Path:     state.Config.Build.Path,
	}
	// Attach the test binary to the request
	if outputs := target.Outputs(); len(outputs) == 1 {
		b, err := ioutil.ReadFile(path.Join(target.OutDir(), outputs[0]))
		if err != nil {
			return nil, nil, nil, err
		}
		request.Binary = &pb.DataFile{Filename: target.Outputs()[0], Contents: b}
	}
	// Attach its runtime files
	for _, datum := range target.Data {
		for _, fullPath := range datum.FullPaths(state.Graph) {
			// Might be a directory, we have to walk it.
			if err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				} else if !info.IsDir() {
					if b, err := ioutil.ReadFile(path); err != nil {
						return err
					} else {
						fn := strings.TrimLeft(strings.TrimPrefix(path, target.OutDir()), "/")
						request.Data = append(request.Data, &pb.DataFile{Filename: fn, Contents: b})
					}
				}
				return nil
			}); err != nil {
				return nil, nil, nil, err
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	response, err := client.Test(ctx, &request)
	if err != nil {
		// N.B. we only get an error here if something failed structurally about the RPC - it is
		//      not an error if we communicate failure in the response.
		return nil, nil, nil, err
	} else if !response.Success {
		return nil, nil, nil, fmt.Errorf("Failed to run test: %s", strings.Join(response.Messages, "\n"))
	} else if !response.ExitSuccess {
		return response.Output, response.Results, response.Coverage, remoteTestFailed
	}
	return response.Output, response.Results, response.Coverage, nil
}

// Dial dials the remote server and returns true if successful.
func (worker *remoteWorker) Dial() bool {
	// TODO(peterebden): One day we will do TLS here.
	conn, err := grpc.Dial(getAddress(config), grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(remoteConnTimeout))
	if s, ok := status.FromError(err); ok && s.Code() == codes.ResourceExhausted {
		// This is a transient error from the remote; it indicates that the server is
		// already busy but we got it anyway. Retry once
		log.Debug("Got busy remote, will retry: %s", s.Err())
		conn, err = grpc.Dial(getAddress(config), grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(remoteConnTimeout))
	}
	if err != nil {
		return false
	}
	worker.client = pb.NewTestWorkerClient(conn)
	return true
}

// Release releases this client back to the worker pool when done.
func (worker *remoteWorker) Release() {
	// It's a little unfortunate that we can't use these, but we need to load balance each request.
	// We might consider using grpclb or something in the future.
	worker.client = nil
	clientsLock.Lock()
	defer clientsLock.Unlock()
	availableRemoteClients = append(availableRemoteClients, *remoteWorker)
}

var availableRemoteClients []remoteWorker
var clientsLock sync.Mutex
var clientsOnce sync.Once

// getRemoteClient returns a client of one of our remote test workers, or nil
// if none is available right now.
func getRemoteClient(config *core.Configuration) *remoteWorker {
	clientsOnce.Do(func() {
		availableRemoteClients = make([]remoteWorker, len(config.Test.NumRemoteWorkers))
		for i := range availableRemoteClients {
			availableRemoteClients[i].id = i
		}
	})
	clientsLock.Lock()
	if len(availableRemoteClients == 0) {
		clientsLock.Unlock()
		return nil
	}
	client := &availableRemoteClients[0]
	availableRemoteClients = availableRemoteClients[1:]
	clientsLock.Unlock()
	// Establish a connection to the remote server before we give this guy back.
	if !client.Dial() {
		// Didn't work, release the client.
		client.Release()
		return nil
	}
	return client
}

// getAddress returns a remote address from the given config.
func getAddress(config *core.Configuration) string {
	if len(config.Test.RemoteWorker) == 1 {
		return config.Test.RemoteWorker[0]
	}
	return config.Test.RemoteWorker[rand.Intn(len(config.Test.RemoteWorker))].String()
}

// canRunRemotely returns true if the given target can be run on a remote worker.
func canRunRemotely(config *core.Configuration, target *core.BuildTarget) bool {
	return config.Test.NumRemoteWorkers == 0 &&
		!target.HasAnyLabel(config.Test.LocalLabels) &&
		(len(config.Test.RemoteLabels) == 0 || target.HasAnyLabel(config.Test.LocalLabels))
}
