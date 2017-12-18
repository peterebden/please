// +build proto

package test

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"core"
	pb "test/proto/worker"
)

const remoteConnTimeout = 2 * time.Second

type remoteCoordinator struct {
	sync.Mutex
	workers []*remoteWorker
	init    sync.Once
	client  pb.TestCoordinatorClient
}

type remoteWorker struct {
	client pb.TestWorkerClient
	id     int
}

var coordinator remoteCoordinator

// RunRemotely runs a single test on a remote worker.
func (worker *remoteWorker) RunRemotely(state *core.BuildState, target *core.BuildTarget) {
	output, results, coverage, err := worker.runRemotely(state, target)
	if err != nil {
		// Failed
	}
}

// runRemotely does the real work of RunRemotely and returns the output, test results, coverage and any error encountered.
func (worker *remoteWorker) runRemotely(state *core.BuildState, target *core.BuildTarget) (output, results, coverage []byte, err error) {
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

// GetRemoteWorker returns a remote client to be used for this target.
// TODO(peterebden): eventually we may want to be able to reuse one for future targets too
//                   to save on the setup / communication costs.
func (coordinator *remoteCoordinator) GetRemoteWorker(config *core.Configuration, target *core.BuildTarget) *remoteWorker {
	coordinator.init.Do(func() {
		conn, err := grpc.Dial(config.Test.RemoteWorker, grpc.WithInsecure(), grpc.WithTimeout(remoteConnTimeout))
		if err != nil {
			log.Warning("Failed to connect to remote test coordinator: %s", err)
			return
		}
		coordinator.client = pb.NewTestCoordinatorClient(conn)
		coordinator.workers = make([]*remoteWorker, config.Test.NumRemoteWorkers)
	})
	// Handle failure from init func
	if coordinator.client == nil {
		return nil
	}
	// Take a worker slot
	worker := coordinator.getAvailableSlot()
	if worker == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), remoteConnTimeout)
	defer cancel()
	response, err := coordinator.client.GetClient(ctx, &ClientRequest{
		Rule: &pb.BuildLabel{
			PackageName: target.Label.PackageName,
			Name:        target.Label.Name,
		},
		Labels: target.Labels,
	})
	if err != nil {
		log.Warning("Failed to communicate with remote test coordinator: %s", err)
		coordinator.releaseSlot(worker)
		return nil
	} else if !response.Success {
		log.Info("Remote test worker not available, will run locally.")
		coordinator.releaseSlot(worker)
		return nil
	}
	// Dial the client here; grpc-go's Dial is non-blocking so it's easier to handle this way.
	conn, err := grpc.Dial(response.URL, grpc.WithInsecure(), grpc.WithTimeout(remoteConnTimeout))
	if err != nil {
		log.Warning("Failed to dial remote worker: %s", err)
		coordinator.releaseSlot(worker)
		return nil
	}
	worker.client = pb.NewTestWorkerClient(conn)
	return worker
}

// getSlot returns an available remote worker, or nil if none is available.
func (coordinator *remoteCoordinator) getSlot() *remoteWorker {
	coordinator.Lock()
	defer coordinator.Unlock()
	for i, w := range coordinator.workers {
		if w == nil {
			w = &remoteWorker{id: i}
			coordinator.workers[i] = w
			return w
		}
	}
	return nil
}

// releaseSlot returns a previously received remote worker to the pool.
func (coordinator *remoteCoordinator) releaseSlot(w *remoteWorker) {
	coordinator.workers[w.id] = nil
}

// canRunRemotely returns true if the given target can be run on a remote worker.
func canRunRemotely(config *core.Configuration, target *core.BuildTarget) bool {
	return config.Test.NumRemoteWorkers == 0 &&
		!target.HasAnyLabel(config.Test.LocalLabels) &&
		(len(config.Test.RemoteLabels) == 0 || target.HasAnyLabel(config.Test.LocalLabels))
}
