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

	"core"
	pb "test/proto/worker"
)

// runTestRemotely runs a single test against a remote worker and returns the output, results & any error.
func runTestRemotely(state *core.BuildState, target *core.BuildTarget) ([]byte, [][]byte, []byte, error) {
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

type clientManager struct {
	sync.RWMutex
	clients map[string]pb.TestWorkerClient
}

var manager = clientManager{clients: map[string]pb.TestWorkerClient{}}

// GetClient returns a client of one of our remote test workers.
// TODO(pebers): we lazy-initialise these, doing it eagerly at startup would probably be a
//               small optimisation so they're dialed and ready when we want to use them.
func (m *clientManager) GetClient(config *core.Configuration) (pb.TestWorkerClient, error) {
	address := config.Test.RemoteWorker[rand.Intn(len(config.Test.RemoteWorker))].String()
	m.RLock()
	client, present := m.clients[address]
	m.RUnlock()
	if present {
		return client, nil
	}
	// Need to initialise a new one.
	// Technically this is a little racy but there is little harm creating an extra client.
	m.Lock()
	defer m.Unlock()
	// TODO(pebers): Support secure connections here.
	conn, err := grpc.Dial(address, grpc.WithTimeout(5*time.Second), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	client = pb.NewTestWorkerClient(conn)
	m.clients[address] = client
	return client, nil
}
