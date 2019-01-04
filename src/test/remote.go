// +build !bootstrap

package test

import (
	"context"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"github.com/thought-machine/please/src/core"
	"github.com/thought-machine/please/src/grpcutil"
	pb "github.com/thought-machine/please/src/remote/proto/remote"
)

var remoteClient pb.RemoteWorkerClient
var remoteClientOnce sync.Once

func runTestRemotely(tid int, state *core.BuildState, target *core.BuildTarget) (core.TestSuite, error) {
	state.LogBuildResult(tid, target.Label, core.TargetTesting, "Contacting remote server...")
	remoteClientOnce.Do(func() {
		// TODO(peterebden): Add TLS support (as always)
		// TODO(peterebden): we should share one of these with the build code as well.
		conn := grpcutil.Dial(state.Config.Build.RemoteURL)
		remoteClient = pb.NewRemoteWorkerClient(conn)
	})
	_, cancel := context.WithTimeout(context.Background(), testTimeout(state.Config, target))
	defer cancel()
	return core.TestSuite{}, nil
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
