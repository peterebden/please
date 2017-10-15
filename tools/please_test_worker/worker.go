package worker

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"sync"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"gopkg.in/op/go-logging.v1"

	"core"
	"test"
	pb "test/proto/worker"
)

var log = logging.MustGetLogger("please_test_worker")

// A Worker implements our remote test worker protocol.
// Note that it will only execute one test simultaneously. Further RPCs will block until the previous ones are complete.
type Worker struct {
	sync.Mutex
}

func (w *Worker) Test(ctx context.Context, req *pb.TestRequest) (*pb.TestResponse, error) {
	w.Lock()
	defer w.Unlock()
	// Build a sufficient representation of the target that we can call the preexisting code for this.
	state := core.NewBuildState(1, nil, 2, core.DefaultConfiguration())
	target := core.NewBuildTarget(core.BuildLabel{PackageName: req.Rule.PackageName, Name: req.Rule.Name})
	target.TestCommand = req.Command
	state.NeedCoverage = req.Coverage
	state.TestArgs = req.TestName
	state.Config.Build.Path = req.Path
	target.NoTestOutput = req.NoOutput
	dir := target.TestDir()
	log.Notice("Received test request for %s", target.Label)

	// From here on, if anything goes wrong, remove the temp directory.
	defer w.cleanup(dir)

	// Write the required files
	if req.Binary != nil && req.Binary.Filename != "" {
		target.AddOutput(req.Binary.Filename)
		if err := w.writeFile(dir, req.Binary); err != nil {
			return w.error("Failed to write test file: %s", err)
		}
	}
	for _, df := range req.Data {
		if err := w.writeFile(dir, df); err != nil {
			return w.error("Failed to write test data file: %s", err)
		}
	}
	out, results, coverage, err := test.RunTest(state, target)
	if err != nil {
		log.Error("Test failed: %s %s", err, out)
	}
	return &pb.TestResponse{
		Rule:        req.Rule,
		Success:     true,
		ExitSuccess: err == nil,
		Output:      out,
		Results:     results,
		Coverage:    coverage,
	}, nil
}

// writeFile is a convenience wrapper to create one of the test files & any needed directories.
func (w *Worker) writeFile(dir string, df *pb.DataFile) error {
	// Note that we must use the target's test dir for the normal code to work.
	// We might want a slightly flatter structure here but on the other hand it makes it easy to
	// run these workers from the root of a Please repo without polluting anything else.
	filename := path.Join(dir, df.Filename)
	if err := os.MkdirAll(path.Dir(filename), core.DirPermissions); err != nil {
		return err
	}
	log.Notice("Writing temp test file %s", filename)
	return ioutil.WriteFile(filename, df.Contents, 0755)
}

// error produces a response proto containing given error text.
// Note that it does not produce an error object, that should only be used for the
// RPC itself failing.
func (w *Worker) error(msg string, err error) (*pb.TestResponse, error) {
	return &pb.TestResponse{
		Success:  false,
		Messages: []string{fmt.Sprintf(msg, err)},
	}, nil
}

// cleanup removes the temporary test directory.
// Note that we can't return an error since it's deferred (and strictly doesn't represent an error to
// run the test anyway).
func (w *Worker) cleanup(dir string) {
	if err := os.RemoveAll(dir); err != nil {
		log.Error("Failed to remove temporary test directory: %s", err)
	}
}

// ServeGrpcForever starts a new server on the given port and serves gRPC until killed.
func ServeGrpcForever(port, maxMsgSize int) {
	s, lis := startGrpcServer(port, maxMsgSize)
	s.Serve(lis)
}

// startGrpcServer starts a gRPC server on the given port and returns it.
func startGrpcServer(port, maxMsgSize int) (*grpc.Server, net.Listener) {
	repoRoot, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to determine working directory: %s", err)
	}
	core.RepoRoot = repoRoot // This needs to be set for later.
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("%s", err)
	}
	s := grpc.NewServer(grpc.MaxMsgSize(maxMsgSize))
	pb.RegisterTestWorkerServer(s, &Worker{})
	log.Notice("Serving test worker on port %d", port)
	return s, lis
}
