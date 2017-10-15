package test

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"cli"
	"core"
	pb "test/proto/worker"
)

var serverAddress string

const results = "Result: Success!"
const coverage = "nope, no coverage :("

func init() {
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatalf("%s", err)
	}
	s := grpc.NewServer()
	pb.RegisterTestWorkerServer(s, &worker{})
	go s.Serve(lis)
	serverAddress = lis.Addr().String()
}

func TestRunTestRemotelyNoResults(t *testing.T) {
	state, target := getState("//package:test_run_test_remotely")
	target.NoTestOutput = true
	out, _, _, err := runTestRemotely(state, target)
	assert.NoError(t, err)
	assert.Equal(t, []byte("ok"), out)
}

func TestRunTestRemotelyResults(t *testing.T) {
	state, target := getState("//package:test_results")
	out, r, c, err := runTestRemotely(state, target)
	assert.NoError(t, err)
	assert.Equal(t, []byte("ok"), out)
	assert.Equal(t, [][]byte{[]byte(results)}, r)
	assert.Equal(t, []byte(coverage), c)
}

func TestRunTestRemotelyData(t *testing.T) {
	state, target := getState("//package:test_data")
	target.Data = append(target.Data, target.Label)
	out, _, _, err := runTestRemotely(state, target)
	assert.NoError(t, err)
	assert.Equal(t, []byte("ok"), out)
}

func TestRunRemotelyRPCError(t *testing.T) {
	state, target := getState("//package:test_rpc_error")
	target.NoTestOutput = true
	_, _, _, err := runTestRemotely(state, target)
	assert.Error(t, err)
}

type worker struct{}

func (w *worker) Test(ctx context.Context, req *pb.TestRequest) (*pb.TestResponse, error) {
	if req.Rule.Name == "test_run_test_remotely" {
		return &pb.TestResponse{
			Rule:        req.Rule,
			Success:     true,
			Output:      []byte("ok"),
			ExitSuccess: true,
		}, nil
	} else if req.Rule.Name == "test_results" {
		return &pb.TestResponse{
			Rule:        req.Rule,
			Success:     true,
			Output:      []byte("ok"),
			Results:     [][]byte{[]byte(results)},
			Coverage:    []byte(coverage),
			ExitSuccess: true,
		}, nil
	} else if req.Rule.Name == "test_data" {
		if len(req.Data) == 0 {
			return nil, fmt.Errorf("Missing data")
		}
		return &pb.TestResponse{
			Rule:        req.Rule,
			Success:     true,
			Output:      []byte("ok"),
			ExitSuccess: true,
		}, nil
	}
	return nil, fmt.Errorf("unknown target: %s", req.Rule.Name)
}

func getState(label string) (*core.BuildState, *core.BuildTarget) {
	state := core.NewBuildState(1, nil, 3, core.DefaultConfiguration())
	state.Config.Test.RemoteWorker = []cli.URL{cli.URL(serverAddress)}
	target := core.NewBuildTarget(core.ParseBuildLabel(label, ""))
	state.Graph.AddTarget(target)
	target.AddOutput(target.Label.Name)
	os.MkdirAll(target.TestDir(), core.DirPermissions)
	os.MkdirAll(target.OutDir(), core.DirPermissions)
	ioutil.WriteFile(path.Join(target.OutDir(), target.Outputs()[0]), []byte("binary"), 0755)
	return state, target
}
