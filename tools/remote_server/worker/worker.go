package worker

import (
	"context"
	"io/ioutil"
	"path"
	"sort"
	"strings"

	"github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"
	"gopkg.in/op/go-logging.v1"

	"build"
	"core"
	"fs"
	"grpcutil"
	pb "test/proto/remote"
)

var log = logging.MustGetLogger("worker")

// Connect connects to the master and receives messages.
// It continues forever until the server terminates.
func Connect(master, name, url string, port int, dir string) {
	// Start the local gRPC server first.
	w := &worker{Dir: path.Join(dir, "test")}
	go w.Start(port)

	conn, err := grpc.Dial(master, grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(grpc_retry.WithMax(3))))
	if err != nil {
		log.Fatalf("Failed to dial server: %s", err)
	}
	client := pb.NewRemoteTestMasterClient(conn)
	stream, err := client.ConnectWorker(context.Background())
	if err != nil {
		log.Fatalf("Failed to connect: %s", err)
	}
	err = stream.Send(&pb.ConnectWorkerRequest{
		Name: name,
		Url:  url,
	})
	if err != nil {
		log.Fatalf("Failed to connect: %s", err)
	}
	worker.Client = client
	// We won't receive a response until the server stops.
	resp, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("Stream terminated: %s", err)
	}
	// If we get here it's a non-fatal response so we exit gently.
	log.Notice("Stream terminated: %s", resp.Error)
}

type worker struct {
	Dir     string
	Client  pb.RemoteTestMasterClient
	Running bool
}

// Start starts serving the worker gRPC server on the given port.
func (w *worker) Start(port int) {
	s := grpc.NewServer()
	pb.RegisterRemoteTestWorkerServer(s, w)
	grpcutil.StartServer(s, port)
}

// ExecuteTest implements the RemoteTestWorker RPC service.
func (w *worker) ExecuteTest(ctx context.Context, req *pb.RemoteTestRequest) (*pb.RemoteTestResponse, error) {
	if w.Running {
		return &pb.RemoteTestResponse{Error: "worker busy"}, nil
	}
	// We could do this with a mutex or whatever, but we don't want callers to wait; this
	// should never happen (because the master should arbitrate the requests).
	w.Running = true
	defer func() {
		w.Running = false
	}()
	stderr, err := w.runTest(ctx, req)
	resp := &pb.RemoteTestResponse{
		Success: err == nil,
		Results: w.findResults(req),
		Stderr:  stderr,
	}
	if err != nil {
		resp.Error = err.Error()
	}
	return resp, nil
}

// runTest runs a single test.
func (w *worker) runTest(ctx context.Context, req *pb.RemoteTestRequest) ([]byte, error) {
	if err := os.RemoveAll(w.Dir); err != nil {
		return nil, err
	} else if err := os.MkdirAll(w.Dir, os.ModeDir|0755); err != nil {
		return nil, err
	} else if err := w.setupFiles(req.Outputs, req.Data); err != nil {
		return nil, err
	}
	// Using a default state here is not really accurate but is the best we can do for now.
	// TODO(peterebden): Find some way of specifying this on the worker.
	target := core.NewBuildTarget(core.BuildLabel{
		PackageName: req.Label.PackageName,
		Name:        req.Label.Name,
	})
	// Set up target files - these become env vars later.
	for _, output := range orderedFiles(req.Outputs) {
		target.AddOutput(output)
	}
	for _, datum := range orderedFiles(req.Data) {
		target.AddDatum(core.FileLabel{File: datum})
	}
	state := core.DefaultBuildState()
	state.NeedCoverage = req.Coverage
	cmd := build.ReplaceTestSequences(state, target, req.Command)
	env := core.TestEnvironment(state, target, w.Dir)
	timeout := time.Duration(req.Timeout) * time.Second
	log.Notice("Running test %s\nENVIRONMENT:\n%s\n%s", target.Label, strings.Join(env, "\n"), cmd)
	// TODO(peterebden): The second-to-last argument allows sandboxing. We should probably allow
	//                   configuring that on the build agent.
	_, stderr, err := core.ExecWithTimeoutShellStdStreams(state, target, w.Dir, env, timeout, timeout, false, cmd, false, false)
	return stderr, err
}

// setupFiles creates the files in the test directory.
func (w *worker) setupFiles(files ...map[string][]byte) error {
	for _, m := range files {
		for filename, contents := range m {
			if err := ioutil.WriteFile(path.Join(w.Dir, filename), contents, 0755); err != nil {
				return err
			}
		}
	}
	return nil
}

// orderedFiles returns the keys of one of the proto file maps, in order.
func (w *worker) orderedFiles(in map[string][]byte) []string {
	ret := make([]string, 0, len(in))
	for file := range in {
		ret = append(ret, file)
	}
	sort.Strings(ret)
	return ret
}

// findResults reads the results files for a test.
func (w *worker) findResults() map[string][]byte {
	ret := map[string][]byte{}
	fs.Walk(w.Dir, func(name string, isDir bool) {
		if !isDir {
			b, _ := ioutil.ReadFile(name)
			ret[name] = b
		}
	})
	return ret
}
